package service

import (
	"fmt"
	"github.com/ahmetson/client-lib"
	clientConfig "github.com/ahmetson/client-lib/config"
	"github.com/ahmetson/config-lib/service"
	"github.com/ahmetson/datatype-lib/data_type/key_value"
	"github.com/ahmetson/datatype-lib/message"
	"github.com/ahmetson/handler-lib/base"
	handlerConfig "github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/replier"
	"github.com/ahmetson/handler-lib/sync_replier"
	"slices"
	"sync"
)

type RequestHandleFunc = func(handlerId string, req message.RequestInterface) (message.RequestInterface, error)
type ReplyHandleFunc = func(handlerId string, req message.RequestInterface, reply message.ReplyInterface) (message.ReplyInterface, error)

// Proxy defines the parameters of the proxy parent
type Proxy struct {
	*Auxiliary
	rule            *service.Rule // set it if this proxy is first in the chain
	proxyConf       *service.Proxy
	onRequest       RequestHandleFunc
	onReply         ReplyHandleFunc
	handlerWrappers map[string]*HandlerWrapper
	handlers        map[handlerConfig.HandlerType]func() base.Interface // todo add support of the trigger
}

type HandlerWrapper struct {
	destConfig *handlerConfig.Handler
	destClient *client.Socket
}

// NewProxy proxy parent returned
func NewProxy() (*Proxy, error) {
	auxiliary, err := NewAuxiliary()
	if err != nil {
		return nil, fmt.Errorf("parent.NewAuxiliary: %w", err)
	}

	auxiliary.Type = service.ProxyType

	handlers := make(map[handlerConfig.HandlerType]func() base.Interface, 0)
	handlers[handlerConfig.SyncReplierType] = func() base.Interface {
		return sync_replier.New()
	}
	handlers[handlerConfig.ReplierType] = func() base.Interface {
		return replier.New()
	}

	return &Proxy{
		auxiliary,
		nil,
		nil,
		nil,
		nil,
		make(map[string]*HandlerWrapper),
		handlers,
	}, nil
}

// The routeWrapper is the proxy route that's invoked for all proxy units.
// The route wrapper calls user functions for the requests or replies.
func (proxy *Proxy) routeWrapper(handlerId string, req message.RequestInterface) message.ReplyInterface {
	handlerWrapper, ok := proxy.handlerWrappers[handlerId]
	if !ok {
		return req.Fail(fmt.Sprintf("internal error, proxy.handlerWrappers[%s] not found", handlerId))
	}

	var nextReq message.RequestInterface
	if proxy.onRequest != nil {
		parsedReq, err := proxy.onRequest(handlerId, req)
		// check failed
		if err != nil {
			return req.Fail(fmt.Sprintf("onRequest(%s): %v", handlerId, err))
		}
		nextReq = parsedReq
		nextReq.SetConId(req.ConId())
	} else {
		nextReq = req
	}
	if !handlerConfig.CanReply(handlerWrapper.destConfig.Type) {
		err := handlerWrapper.destClient.Submit(nextReq)
		if err != nil {
			return nextReq.Fail(fmt.Sprintf("handler %s not replieable, submit failed as for req %v: %v", handlerId, nextReq, err))
		}
		return nextReq.Ok(key_value.New())
	}
	reply, err := handlerWrapper.destClient.Request(nextReq)
	if err != nil {
		return nextReq.Fail(fmt.Sprintf("handlerWrapper.destClient(handlerId='%s', req=%v): %v", handlerId, nextReq, err))
	}
	if proxy.onReply == nil {
		reply.SetConId(req.ConId())
		return reply
	}

	parsedReply, err := proxy.onReply(handlerId, nextReq, reply)
	// check failed
	if err != nil {
		return nextReq.Fail(fmt.Sprintf("onReply(handlerId='%s', 'request'='%v', reply='%v'): %v", handlerId,
			req, reply, err))
	}
	parsedReply.SetConId(nextReq.ConId())

	return parsedReply
}

func (proxy *Proxy) SetHandlerDefiner(handlerType handlerConfig.HandlerType, definer func() base.Interface) {
	proxy.handlers[handlerType] = definer
}

// SetHandler is disabled as the proxy returns them from the parent
func (proxy *Proxy) SetHandler(_ string, _ base.Interface) {}

// SetRequestHandler sets the requests function defined by the user.
func (proxy *Proxy) SetRequestHandler(onRequest RequestHandleFunc) error {
	if proxy.onRequest != nil {
		return fmt.Errorf("already set")
	}
	proxy.onRequest = onRequest
	return nil
}

// SetReplyHandler sets the reply function defined by the user.
func (proxy *Proxy) SetReplyHandler(onReply ReplyHandleFunc) error {
	if proxy.onReply != nil {
		return fmt.Errorf("already set")
	}
	proxy.onReply = onReply
	return nil
}

// Todo maybe to call routeHandlers after setting the config?
// So that handlers will have their own generated id?
func (proxy *Proxy) routeHandlers(units []*service.Unit) error {
	// Set up the route for each handler
	for _, unitRef := range units {
		// todo make sure if unit is changed, then unitRef is called by reference
		unit := *unitRef // copy

		handlerWrapper, ok := proxy.handlerWrappers[unit.HandlerId]
		if !ok {
			proxy.Logger.Warn("unit handler wrapper not found", "unit", unit, "handler wrappers", proxy.handlerWrappers,
				"handlers", proxy.Handlers)
			continue
		}
		category := proxy.id + handlerWrapper.destConfig.Category
		handler, ok := proxy.Handlers[category].(base.Interface)
		if !ok {
			return fmt.Errorf(fmt.Sprintf("unit handler by category not found, category=%s, wrappers amount=%d, handler id='%s'",
				category, len(proxy.handlerWrappers), unit.HandlerId))
		}

		err := handler.Route(unit.Command, func(request message.RequestInterface) message.ReplyInterface {
			return proxy.routeWrapper(unit.HandlerId, request)
		})
		if err != nil {
			return fmt.Errorf("handler.Route(unit=%v): %w", unit, err)
		}
	}

	return nil
}

// The setProxyUnits prepares the proxy chains by fetching the proxies from the parent
// and storing them in this proxy.
//
// It won't check against nil parameters since it's a private method.
//
// Call it after Proxy.lintProxyChains.
func (proxy *Proxy) setProxyUnits() error {
	proxyClient := proxy.ctx.ProxyClient()
	proxyChains, err := proxyClient.ProxyChains() // returns the linted proxies.
	if err != nil {
		return fmt.Errorf("proxyClient.ProxyChains: %w", err)
	}

	parentClient := proxy.ParentManager

	// set the proxy destination units for each rule
	for _, proxyChain := range proxyChains {
		// the last proxy in the list is removed as its this parent
		rule := proxyChain.Destination

		// For proxy chains set specifically for this proxy, then simply get the proxies
		if slices.Contains(rule.Urls, proxy.url) {
			err := proxy.setProxyUnitsBy(rule)
			if err != nil {
				return fmt.Errorf("proxy.setProxyUnitsBy(rule='%v'): %w", rule, err)
			}
			continue
		}

		units, err := parentClient.Units(rule)
		if err != nil {
			return fmt.Errorf("destClient.Units('%v'): %w", rule, err)
		}
		if err := proxyClient.SetUnits(rule, units); err != nil {
			return fmt.Errorf("proxyClient.SetUnits('%v'): %w", rule, err)
		}

		err = proxy.routeHandlers(units)
		if err != nil {
			return fmt.Errorf("proxy.routeHandlers(rule='%v' from proxy chain=%v): %w", rule, proxyChain, err)
		}
	}

	if proxy.rule != nil {
		rule := proxy.rule

		units, err := parentClient.Units(rule)
		if err != nil {
			return fmt.Errorf("destClient.Units('%v'): %w", rule, err)
		}
		if err := proxyClient.SetUnits(rule, units); err != nil {
			return fmt.Errorf("proxyClient.SetUnits('%v'): %w", rule, err)
		}

		err = proxy.routeHandlers(units)
		if err != nil {
			return fmt.Errorf("proxy.routeHandlers(rule='%v' from proxy.rule): %w", rule, err)
		}
	}

	return nil
}

// The lintProxyChain method fetches the proxy chain and a rule from the parent.
// Then set it in this Proxy context.
// Todo, make sure to listen for the proxy parameters from the parent by a loop.
func (proxy *Proxy) lintProxyChain() error {
	// first, get the proxy chain parameter for this proxy chain
	proxyChains, err := proxy.ParentManager.ProxyChainsByLastProxy(proxy.id)
	if err != nil {
		return fmt.Errorf("parentManager.ProxyChainsByLastProxy(id='%s'): %w", proxy.id, err)
	}
	if len(proxyChains) == 0 {
		return fmt.Errorf("parentManager.ProxyChainsByLastProxy(id='%s'): empty proxy chains", proxy.id)
	}
	proxyChain := proxyChains[0]
	if proxyChain.Sources == nil {
		proxyChain.Sources = []string{}
	}

	if !proxyChain.IsValid() {
		return fmt.Errorf("parentManager.ProxyChainsByLastProxy(id='%s'): proxy chain is not valid", proxy.id)
	}

	preLast := len(proxyChain.Proxies) - 1
	proxy.proxyConf = proxyChain.Proxies[preLast]
	proxies := make([]*service.Proxy, 0, preLast)
	proxies = append(proxies, proxyChain.Proxies[:preLast]...)
	proxyChain.Proxies = proxies

	// No proxy chain, it's the first proxy chain
	if len(proxyChain.Proxies) == 0 {
		proxy.rule = proxyChain.Destination
		return nil
	}

	// the rule will be stored in the proxy handler manager
	proxy.rule = nil

	// Add to the proxy client queue the proxy chain.
	// When the proxy will start the base service, the proxy handler will fetch it.
	err = proxy.SetProxyChain(proxyChain)
	if err != nil {
		return fmt.Errorf("proxy.SetProxyChain(rule='%v', id='%s'): %w", proxyChain.Destination, proxy.id, err)
	}

	return nil
}

// For now, this method supports one rule, as the proxies support one destination for now.
func (proxy *Proxy) destination() (*service.Rule, error) {
	if proxy.rule != nil {
		return proxy.rule, nil
	}

	proxyClient := proxy.ctx.ProxyClient()
	proxyChains, err := proxyClient.ProxyChains()
	if err != nil {
		return nil, fmt.Errorf("proxyClient.ProxyChainsByRuleUrl: %w", err)
	}
	if len(proxyChains) == 0 {
		return nil, fmt.Errorf("proxyClient.ProxyChains: 0 proxy chains")
	}

	return proxyChains[0].Destination, nil
}

// The lintHandlers method fetches the handlers from the parent.
// Based on the handlers, it creates this proxy's handlers.
//
// Todo handlers must route to the proxy.RequestHandler.
// Todo, make sure to listen for the proxy parameters from the parent by a loop.
//
// Proxy supports:
//   - replier
//   - sync_replier
func (proxy *Proxy) lintHandlers() error {
	destination, err := proxy.destination()
	if err != nil {
		return fmt.Errorf("proxy.destination: %w", err)
	}

	handlerConfigs, err := proxy.ParentManager.HandlersByRule(destination)
	if err != nil {
		return fmt.Errorf("proxy.ParentManager.HandlersByRule(rule='%v', parentId='%s'): %w", destination, proxy.id, err)
	}
	if len(handlerConfigs) == 0 {
		return fmt.Errorf("proxy.ParentManager.HandlersByRule(rule='%v', parentId='%s'): no handler configs", destination, proxy.id)
	}
	slices.CompactFunc(handlerConfigs, func(x, y *handlerConfig.Handler) bool {
		return x.Id == y.Id
	})

	for i := range handlerConfigs {
		definer, ok := proxy.handlers[handlerConfigs[i].Type]
		if !ok {
			unknown, unknownOk := proxy.handlers[handlerConfig.UnknownType]
			if !unknownOk {
				return fmt.Errorf("the handler config %d of %s type has no handler definer", i, handlerConfigs[i].Type)
			}
			definer = unknown
		}
		h := definer()
		// todo use the proxy category; when generating a proxy parentId,
		// it needs to over-write the generateConfig method of the parent to set a new parentId.
		proxy.Auxiliary.SetHandler(proxy.id+handlerConfigs[i].Category, h)

		// could lead to unexpected behavior if there are multiple urls
		parentZmqType := handlerConfig.SocketType(handlerConfigs[i].Type)
		parentClientConf := clientConfig.New(destination.Urls[0], handlerConfigs[i].Id, handlerConfigs[i].Port, parentZmqType)
		parentClientConf.UrlFunc(clientConfig.Url)
		parentClient, err := client.New(parentClientConf)
		if err != nil {
			return fmt.Errorf("client.New(parentClientConf='%v'): failed to create a destination socket: %w", *parentClientConf, err)
		}

		handlerWrapper := &HandlerWrapper{
			destConfig: handlerConfigs[i],
			destClient: parentClient,
		}
		proxy.handlerWrappers[handlerConfigs[i].Id] = handlerWrapper
	}

	return nil
}

// Start the proxy.
//
// Proxy can start without the parent.
// And when a parent starts, it will fetch the parameters.
// Todo make sure that proxy chain update in a live mode affects the Service.
func (proxy *Proxy) Start() (*sync.WaitGroup, error) {
	proxy.ctx.SetService(proxy.id, proxy.url)
	if !proxy.ctx.IsDepManagerRunning() {
		if err := proxy.ctx.StartDepManager(); err != nil {
			err = fmt.Errorf("ctx.StartDepManager: %w", err)
			if closeErr := proxy.ctx.Close(); closeErr != nil {
				return nil, fmt.Errorf("%v: cleanout context: %w", err, closeErr)
			}
			return nil, err
		}
	}

	if !proxy.ctx.IsProxyHandlerRunning() {
		if err := proxy.ctx.StartProxyHandler(); err != nil {
			err = fmt.Errorf("ctx.StartProxyHandler: %w", err)
			if closeErr := proxy.ctx.Close(); closeErr != nil {
				return nil, fmt.Errorf("%v: cleanout context: %w", err, closeErr)
			}
			return nil, err
		}
	}

	err := proxy.lintProxyChain()
	if err != nil {
		return nil, fmt.Errorf("proxy.lintProxyChain: %w", err)
	}

	// get the list of the handlers if there is no given in the handler list
	err = proxy.lintHandlers()
	if err != nil {
		return nil, fmt.Errorf("proxy.lintHandlers: %w", err)
	}
	if err = proxy.setConfig(); err != nil {
		return nil, fmt.Errorf("proxy.SetConfig: %w", err)
	}

	// get the proxies from the proxy chain for this service.
	// must be called before starting handlers, as routing of the handlers maybe set by proxy units.
	if err = proxy.setProxyUnits(); err != nil {
		return nil, fmt.Errorf("proxy.setProxyUnits: %w", err)
	}

	// todo call the setConfig first then invoke the ParentManager.SetProxyChain
	// then start the auxiliary.
	// Because auxiliary will start the proxies as well.
	// We don't want to block until the proxies are set to indicate the parent.
	wg, err := proxy.Auxiliary.Start()
	if err != nil {
		return nil, fmt.Errorf("proxy.Auxiliary.Start: %w", err)
	}

	// send to the parent info that it was set.
	rule, _ := proxy.destination()
	if rule != nil {
		serviceConf, err := proxy.ctx.Config().Service(proxy.id)
		if err != nil {
			return wg, fmt.Errorf("proxy.ctx.Config().Service(id='%s'): %w", proxy.id, err)
		}

		source := &service.SourceService{
			Proxy:   proxy.proxyConf,
			Manager: serviceConf.Manager,
			Clients: make([]*clientConfig.Client, len(serviceConf.Handlers)),
		}
		for i := range serviceConf.Handlers {
			handlerConf := serviceConf.Handlers[i]
			handlerZmqType := handlerConfig.SocketType(handlerConf.Type)
			clientConf := clientConfig.New(proxy.url, handlerConf.Id, handlerConf.Port, handlerZmqType)

			source.Clients[i] = clientConf
		}
		err = proxy.ParentManager.ProxyConfigSet(rule, source)
		if err != nil {
			return wg, fmt.Errorf("proxy.ParentManager.ProxyConfigSet(rule='%v', source='%v'): %w",
				*rule, *source, err)
		}
	}

	return wg, nil
}
