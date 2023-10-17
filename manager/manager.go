// Package manager is the manager of the service.
package manager

import (
	"fmt"
	clientConfig "github.com/ahmetson/client-lib/config"
	serviceConfig "github.com/ahmetson/config-lib/service"
	"github.com/ahmetson/datatype-lib/data_type/key_value"
	"github.com/ahmetson/datatype-lib/message"
	context "github.com/ahmetson/dev-lib"
	"github.com/ahmetson/handler-lib/base"
	handlerConfig "github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/manager_client"
	syncReplier "github.com/ahmetson/handler-lib/sync_replier"
	"sync"
)

const (
	Heartbeat           = "heartbeat"
	Close               = "close"
	ProxyChainsByLastId = "proxy-chains-by-last-id"
	Units               = "units"
	Handlers            = "handlers"             // returns handler configurations
	HandlersByCategory  = "handlers-by-category" // returns the handler configurations by their category
	HandlersByRule      = "handlers-by-rule"     // returns the handler configurations filtered by serviceConfig.Rule
	ProxyConfigSet      = "proxy-config-set"     // proxy calls this route when there configuration was set
)

// The Manager keeps all necessary parameters of the service.
// Manage this service from other parts.
type Manager struct {
	base.Interface
	serviceUrl      string
	serviceId       string
	handlerManagers []manager_client.Interface
	deps            []*clientConfig.Client
	ctx             context.Interface
	blocker         **sync.WaitGroup // block the service
	running         bool
	config          *clientConfig.Client
}

// New service with the parameters.
// Parameter order: id, url, context type
func New(ctx context.Interface, serviceId string, blocker **sync.WaitGroup) (*Manager, error) {
	configClient := ctx.Config()
	returnedConfig, err := configClient.Service(serviceId)
	if err != nil {
		return nil, fmt.Errorf("ctx.Config().Service('%s'): %w", serviceId, err)
	}
	if returnedConfig.Manager == nil {
		return nil, fmt.Errorf("ctx.Config().Service('%s'): Manager field is nil", serviceId)
	}

	handler := syncReplier.New()

	h := &Manager{
		Interface:       handler,
		ctx:             ctx,
		serviceUrl:      returnedConfig.Url,
		serviceId:       serviceId,
		handlerManagers: make([]manager_client.Interface, 0),
		deps:            make([]*clientConfig.Client, 0),
		blocker:         blocker,
		config:          returnedConfig.Manager,
	}

	managerConfig := HandlerConfig(returnedConfig.Manager)
	handler.SetConfig(managerConfig)

	return h, nil
}

// Close the service.
//
// It closes all running handlers.
//
// It closes the dependencies.
//
// It closes the context.
//
// It closes this manager.
//
// It closes all proxies.
func (m *Manager) Close() error {
	serviceConf, err := m.ctx.Config().Service(m.serviceId)
	if err != nil {
		return fmt.Errorf("m.ctx.Config().Service(id='%s'): %w", m.serviceId, err)
	}
	depManager := m.ctx.DepClient()
	for ruleIndex := range serviceConf.Sources {
		for i := range serviceConf.Sources[ruleIndex].Proxies {
			proxy := serviceConf.Sources[ruleIndex].Proxies[i]
			proxy.Manager.UrlFunc(clientConfig.Url)
			err := depManager.CloseDep(proxy.Manager)
			if err != nil {
				return fmt.Errorf("depManager.CloseDep(serviceConf.Sources[%d].Proxies[%d] = %v): %w",
					ruleIndex, i, *proxy, err)
			}
		}
	}

	// closing all handlers
	for _, h := range m.handlerManagers {
		err := h.Close()
		if err != nil {
			return fmt.Errorf("handlerManagers('%s').Close: %v", h.Id(), err)
		}
	}
	m.handlerManagers = make([]manager_client.Interface, 0)

	err = m.ctx.Close()
	if err != nil {
		return fmt.Errorf("ctx.Close: %w", err)
	}

	managerConfig := HandlerConfig(m.config)
	handlerManager, err := manager_client.New(managerConfig)
	if err != nil {
		return fmt.Errorf("manager_client.New: %w", err)
	}
	err = handlerManager.Close()
	if err != nil {
		return fmt.Errorf("handler.Close: %w", err)
	}

	m.running = false
	if m.blocker != nil && *m.blocker != nil {
		fmt.Printf("blocker done!\n")
		(*m.blocker).Done()
	} else {
		fmt.Printf("blocker is nil\n")
	}

	return nil
}

func (m *Manager) Running() bool {
	return m.running
}

// onClose received a close signal for this service
func (m *Manager) onClose(req message.RequestInterface) message.ReplyInterface {
	err := m.Close()
	if err != nil {
		return req.Fail(fmt.Sprintf("manager.Close: %v", err))
	}

	return req.Ok(key_value.New())
}

// onHeartbeat simple handler to check that service is alive
func (m *Manager) onHeartbeat(req message.RequestInterface) message.ReplyInterface {
	return req.Ok(key_value.New())
}

// onProxyChainsByLastProxy returns a list of proxy chains by the id of the last proxy
func (m *Manager) onProxyChainsByLastProxy(req message.RequestInterface) message.ReplyInterface {
	id, err := req.RouteParameters().StringValue("id")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('id'): %v", err))
	}
	proxyClient := m.ctx.ProxyClient()
	proxyChains, err := proxyClient.ProxyChainsByLastId(id)
	if err != nil {
		return req.Fail(fmt.Sprintf("proxyClient.ProxyChainsByLastId('%s'): %v", id, err))
	}

	params := key_value.New().Set("proxy_chains", proxyChains)
	return req.Ok(params)
}

// onUnits returns a list of destination units by a rule
func (m *Manager) onUnits(req message.RequestInterface) message.ReplyInterface {
	raw, err := req.RouteParameters().NestedValue("rule")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('proxy_chain'): %v", err))
	}

	var rule serviceConfig.Rule
	err = raw.Interface(&rule)
	if err != nil {
		return req.Fail(fmt.Sprintf("key_value.KeyValue('proxy_chain').Interface(): %v", err))
	}

	if !rule.IsValid() {
		return req.Fail("the 'rule' parameter is not valid")
	}

	proxyClient := m.ctx.ProxyClient()
	units, err := proxyClient.Units(&rule)
	if err != nil {
		return req.Fail(fmt.Sprintf("proxyClient.Units: %v", err))
	}

	params := key_value.New().Set("units", units)
	return req.Ok(params)
}

// onProxyConfigSet sets the proxy information for a rule as the proxy is set the configuration
func (m *Manager) onProxyConfigSet(req message.RequestInterface) message.ReplyInterface {
	raw, err := req.RouteParameters().NestedValue("rule")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('rule'): %v", err))
	}
	rawSource, err := req.RouteParameters().NestedValue("source_service")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('source_service'): %v", err))
	}

	var rule serviceConfig.Rule
	err = raw.Interface(&rule)
	if err != nil {
		return req.Fail(fmt.Sprintf("key_value.KeyValue('rule').Interface(): %v", err))
	}

	if !rule.IsValid() {
		return req.Fail("the 'rule' parameter is not valid")
	}

	var sourceService serviceConfig.SourceService
	err = rawSource.Interface(&sourceService)
	if err != nil {
		return req.Fail(fmt.Sprintf("key_value.KeyValue('source_service').Interface(): %v", err))
	}

	proxyId := sourceService.Id

	proxyClient := m.ctx.ProxyClient()
	proxyChains, err := proxyClient.ProxyChainsByLastId(proxyId)
	if err != nil {
		return req.Fail(fmt.Sprintf("proxyClient.ProxyChainsByLastId('%s'): %v", proxyId, err))
	}

	proxyChain := serviceConfig.ProxyChainByRule(proxyChains, &rule)
	if proxyChain == nil {
		return req.Fail("the proxy and rule are mismatching")
	}

	configClient := m.ctx.Config()
	c, err := configClient.Service(m.serviceId)
	if err != nil {
		return req.Fail(fmt.Sprintf("configClient.Service('%s'): %v", m.serviceId, err))
	}

	serviceUpdated := c.SetServiceSource(&rule, &sourceService)
	if serviceUpdated {
		err = configClient.SetService(c)
		if err != nil {
			req.Fail(fmt.Sprintf("configClient.SetService: %v", err))
		}
	}

	params := key_value.New()
	return req.Ok(params)
}

// The handlers return the handler configurations
func (m *Manager) handlers() ([]*handlerConfig.Handler, error) {
	handlerConfigs := make([]*handlerConfig.Handler, len(m.handlerManagers))

	for i := range m.handlerManagers {
		handlerManager := m.handlerManagers[i]
		c, err := handlerManager.Config()
		if err != nil {
			return nil, fmt.Errorf("m.handlerManagers[%d]: %w", i, err)
		}

		handlerConfigs[i] = c
	}

	return handlerConfigs, nil
}

// onHandlers returns configuration of the handlers in this service.
//
// If this service is a destination, then the proxy will call this function.
//
// todo, over-write the auxiliary service, so that it will return from the destination.
// todo, auxiliary service must keep the handlers in itself.
func (m *Manager) onHandlers(req message.RequestInterface) message.ReplyInterface {
	handlerConfigs, err := m.handlers()
	if err != nil {
		return req.Fail(fmt.Sprintf("m.handlers: %v", err))
	}

	params := key_value.New().Set("handler_configs", handlerConfigs)
	return req.Ok(params)
}

// onHandlersByCategory returns configuration of the handlers in this service.
//
// If this service is a destination, then the proxy will call this function.
//
// Todo, over-write the auxiliary service, so that it will return from the destination.
// Todo, auxiliary service must keep the handlers in itself.
func (m *Manager) onHandlersByCategory(req message.RequestInterface) message.ReplyInterface {
	category, err := req.RouteParameters().StringValue("category")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().StringValue('category'): %v", err))
	}

	handlerConfigs, err := m.handlers()
	if err != nil {
		return req.Fail(fmt.Sprintf("m.handlers: %v", err))
	}

	filteredConfigs := handlerConfig.ByCategory(handlerConfigs, category)

	params := key_value.New().Set("handler_configs", filteredConfigs)
	return req.Ok(params)
}

// onHandlersByRule returns a list of handler configurations filtered by the serviceConfig.Rule.
func (m *Manager) onHandlersByRule(req message.RequestInterface) message.ReplyInterface {
	raw, err := req.RouteParameters().NestedValue("rule")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('proxy_chain'): %v", err))
	}

	var rule serviceConfig.Rule
	err = raw.Interface(&rule)
	if err != nil {
		return req.Fail(fmt.Sprintf("key_value.KeyValue('proxy_chain').Interface(): %v", err))
	}

	if !rule.IsValid() {
		return req.Fail("the 'rule' parameter is not valid")
	}

	handlerConfigs, err := m.handlers()
	if err != nil {
		return req.Fail(fmt.Sprintf("m.handlers: %v", err))
	}

	if rule.IsService() {
		params := key_value.New().Set("handler_configs", handlerConfigs)
		return req.Ok(params)
	}

	filteredConfigs := make([]*handlerConfig.Handler, 0, len(handlerConfigs))
	for i := range rule.Categories {
		category := rule.Categories[i]
		filteredConfigs = append(filteredConfigs, handlerConfig.ByCategory(handlerConfigs, category)...)
	}

	params := key_value.New().Set("handler_configs", filteredConfigs)
	return req.Ok(params)
}

// HandlerConfig converts the client into the handler configuration
func HandlerConfig(client *clientConfig.Client) *handlerConfig.Handler {
	return &handlerConfig.Handler{
		Type:           handlerConfig.SyncReplierType,
		Category:       serviceConfig.ManagerCategory,
		InstanceAmount: 1,
		Port:           client.Port,
		Id:             client.Id,
	}
}

func (m *Manager) SetHandlerManagers(clients []manager_client.Interface) {
	m.handlerManagers = append(m.handlerManagers, clients...)
}

func (m *Manager) SetDeps(configs []*clientConfig.Client) {
	m.deps = configs
}

// Start the orchestra in the background.
// If it failed to run, then return an error.
// The url request is the main service to which this orchestra belongs too.
func (m *Manager) Start() error {
	if err := m.Route(Close, m.onClose); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, Close, err)
	}
	if err := m.Route(Heartbeat, m.onHeartbeat); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, Heartbeat, err)
	}
	if err := m.Route(ProxyChainsByLastId, m.onProxyChainsByLastProxy); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, ProxyChainsByLastId, err)
	}
	if err := m.Route(Units, m.onUnits); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, Units, err)
	}
	if err := m.Route(Handlers, m.onHandlers); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, Handlers, err)
	}
	if err := m.Route(HandlersByCategory, m.onHandlersByCategory); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, HandlersByCategory, err)
	}
	if err := m.Route(HandlersByRule, m.onHandlersByRule); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, HandlersByRule, err)
	}
	if err := m.Route(ProxyConfigSet, m.onProxyConfigSet); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, ProxyConfigSet, err)
	}

	if err := m.Interface.Start(); err != nil {
		return fmt.Errorf("handler.Start: %w", err)
	}

	m.running = true

	return nil
}
