// Package manager is the manager of the service.
package manager

import (
	"fmt"
	"github.com/ahmetson/client-lib"
	clientConfig "github.com/ahmetson/client-lib/config"
	serviceConfig "github.com/ahmetson/config-lib/service"
	"github.com/ahmetson/datatype-lib/data_type/key_value"
	"github.com/ahmetson/datatype-lib/message"
	context "github.com/ahmetson/dev-lib"
	"github.com/ahmetson/handler-lib/base"
	handlerConfig "github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/manager_client"
	syncReplier "github.com/ahmetson/handler-lib/sync_replier"
	"github.com/ahmetson/log-lib"
	"sync"
)

const (
	Heartbeat           = "heartbeat"
	Close               = "close"
	ProxyChainsByLastId = "proxy-chains-by-last-id"
)

// The Manager keeps all necessary parameters of the service.
type Manager struct {
	serviceUrl     string
	handler        base.Interface // manage this service from other parts. it should be called before the orchestra runs
	handlerManager manager_client.Interface
	handlerClients []manager_client.Interface
	deps           []*clientConfig.Client
	ctx            context.Interface
	blocker        **sync.WaitGroup // block the service
	running        bool
}

type Client struct {
	*client.Socket
}

// New service with the parameters.
// Parameter order: id, url, context type
func New(ctx context.Interface, blocker **sync.WaitGroup, client *clientConfig.Client) (*Manager, error) {
	handler := syncReplier.New()

	h := &Manager{
		ctx:            ctx,
		handler:        handler,
		serviceUrl:     client.ServiceUrl,
		handlerClients: make([]manager_client.Interface, 0),
		deps:           make([]*clientConfig.Client, 0),
		blocker:        blocker,
	}

	managerConfig := h.Config(client)
	handler.SetConfig(managerConfig)

	handlerManager, err := manager_client.New(managerConfig)
	if err != nil {
		return nil, fmt.Errorf("manager_client.New: %w", err)
	}
	h.handlerManager = handlerManager

	return h, nil
}

// NewClient returns a manager client based on the configuration
func NewClient(c *clientConfig.Client) (*Client, error) {
	socket, err := client.New(c)
	if err != nil {
		return nil, fmt.Errorf("client.New: %w", err)
	}

	return &Client{socket}, nil
}

//
// Client methods
//

// Heartbeat sends a command to the parent to make sure that it's live
func (c *Client) Heartbeat() error {
	req := &message.Request{
		Command:    Heartbeat,
		Parameters: key_value.New(),
	}

	reply, err := c.Request(req)
	if err != nil {
		return fmt.Errorf("c.Request(%s): %w", Heartbeat, err)
	}

	if !reply.IsOK() {
		return fmt.Errorf("reply error message: %s", reply.ErrorMessage())
	}

	return nil
}

func (c *Client) ProxyChainsByLastProxy(proxyId string) ([]*serviceConfig.ProxyChain, error) {
	req := &message.Request{
		Command:    ProxyChainsByLastId,
		Parameters: key_value.New().Set("id", proxyId),
	}
	reply, err := c.Request(req)
	if err != nil {
		return nil, fmt.Errorf("c.Request: %w", err)
	}
	if !reply.IsOK() {
		return nil, fmt.Errorf("reply error message: %s", reply.ErrorMessage())
	}

	kvList, err := reply.ReplyParameters().NestedListValue("proxy_chains")
	if err != nil {
		return nil, fmt.Errorf("reply.ReplyParameters().NestedKeyValueList('proxy_chains'): %w", err)
	}

	proxyChains := make([]*serviceConfig.ProxyChain, len(kvList))
	for i, kv := range kvList {
		err = kv.Interface(proxyChains[i])
		if err != nil {
			return nil, fmt.Errorf("kv.Interface(proxyChains[%d]): %w", i, err)
		}
	}

	return proxyChains, nil
}

//
// Manager Handler methods
//

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
// Todo It doesn't close the proxies, which it must close.
func (m *Manager) Close() error {
	// closing all handlers
	for _, handlerClient := range m.handlerClients {
		err := handlerClient.Close()
		if err != nil {
			return fmt.Errorf("handlerManagerClient('%s').Close: %v", handlerClient.Id(), err)
		}
	}

	// closing all dependencies
	depClient := m.ctx.DepManager()
	for _, c := range m.deps {
		if err := depClient.CloseDep(c); err != nil {
			return fmt.Errorf("depClient.CloseDep('%s'): %v", c.Id, err)
		}
	}

	err := m.ctx.Close()
	if err != nil {
		return fmt.Errorf("ctx.Close: %w", err)
	}

	err = m.handlerManager.Close()
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

// Config of the manager
func (m *Manager) Config(client *clientConfig.Client) *handlerConfig.Handler {
	return &handlerConfig.Handler{
		Type:           handlerConfig.SyncReplierType,
		Category:       serviceConfig.ManagerCategory,
		InstanceAmount: 1,
		Port:           client.Port,
		Id:             client.Id,
	}
}

func (m *Manager) SetLogger(parent *log.Logger) error {
	if err := m.handler.SetLogger(parent); err != nil {
		return fmt.Errorf("handler.SetLogger: %w", err)
	}

	return nil
}

func (m *Manager) SetHandlerClients(clients []manager_client.Interface) {
	m.handlerClients = append(m.handlerClients, clients...)
}

func (m *Manager) SetDeps(configs []*clientConfig.Client) {
	m.deps = configs
}

// Start the orchestra in the background.
// If it failed to run, then return an error.
// The url request is the main service to which this orchestra belongs too.
func (m *Manager) Start() error {
	if err := m.handler.Route(Close, m.onClose); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, Close, err)
	}
	if err := m.handler.Route(Heartbeat, m.onHeartbeat); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, Heartbeat, err)
	}
	if err := m.handler.Route(ProxyChainsByLastId, m.onProxyChainsByLastProxy); err != nil {
		return fmt.Errorf(`handler.Route("%s"): %w`, ProxyChainsByLastId, err)
	}

	if err := m.handler.Start(); err != nil {
		return fmt.Errorf("handler.Start: %w", err)
	}

	m.running = true

	return nil
}
