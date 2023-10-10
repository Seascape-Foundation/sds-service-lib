package service

import (
	"fmt"
	"github.com/ahmetson/config-lib/service"
	"github.com/ahmetson/handler-lib/base"
	handlerConfig "github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/replier"
	"github.com/ahmetson/handler-lib/sync_replier"
	"sync"
)

// Proxy defines the parameters of the proxy parent
type Proxy struct {
	*Auxiliary
}

// NewProxy proxy parent returned
func NewProxy() (*Proxy, error) {
	auxiliary, err := NewAuxiliary()
	if err != nil {
		return nil, fmt.Errorf("parent.NewAuxiliary: %w", err)
	}

	auxiliary.Type = service.ProxyType

	return &Proxy{auxiliary}, nil
}

// SetHandler is disabled as the proxy returns them from the parent
func (proxy *Proxy) SetHandler(_ string, _ base.Interface) {}

// The setProxyUnits prepares the proxy chains by fetching the proxies from the parent
// and storing them in this parent.
//
// It won't check against nil parameters since it's a private method.
func (proxy *Proxy) setProxyUnits() error {
	proxyClient := proxy.ctx.ProxyClient()
	proxyChains, err := proxyClient.ProxyChains()
	if err != nil {
		return fmt.Errorf("proxyClient.ProxyChainsByRuleUrl: %w", err)
	}

	parentClient := proxy.ParentManager

	proxy.Logger.Warn("copying proxy chain rule from the parent to the child",
		"warning 1", "the proxy may be over-writing it by adding another units",
		"solution 1", "to the Set and SetUnits of proxy client add an merge flag so it will add to already existing data")

	// set the proxy destination units for each rule
	for _, proxyChain := range proxyChains {
		// the last proxy in the list is removed as its this parent
		proxyChain.Proxies = proxyChain.Proxies[:len(proxyChain.Proxies)-1]
		if len(proxyChain.Proxies) > 0 {
			err := proxyClient.Set(proxyChain)
			if err != nil {
				return fmt.Errorf("proxyClient.Set: %w", err)
			}
		}

		rule := proxyChain.Destination
		units, err := parentClient.Units(rule)
		if err != nil {
			return fmt.Errorf("parentClient.Units: %w", err)
		}
		if err := proxyClient.SetUnits(rule, units); err != nil {
			return fmt.Errorf("proxyClient.SetUnits: %w", err)
		}
	}

	return nil
}

// The lintProxyChain method fetches the proxy url from the parent.
// Then set it in the proxy context.
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
	if !proxyChain.IsValid() {
		return fmt.Errorf("parentManager.ProxyChainsByLastProxy(id='%s'): proxy chain is not valid", proxy.id)
	}
	if len(proxyChain.Proxies) == 0 {
		return fmt.Errorf("parentManager.ProxyChainsByLastProxy(id='%s'): np proxies were given", proxy.id)
	}

	preLast := len(proxyChain.Proxies) - 1
	proxies := make([]*service.Proxy, 0, preLast)
	proxies = append(proxies, proxyChain.Proxies[:preLast]...)
	proxyChain.Proxies = proxies

	err = proxy.SetProxyChain(proxyChain)
	if err != nil {
		return fmt.Errorf("proxy.SetProxyChain(rule='%v', id='%s'): %w", proxyChain.Destination, proxy.id, err)
	}

	return nil
}

// The lintHandlers method fetches the handlers from the parent.
// Then set the handlers in the proxy parent.
//
// Todo handlers must route to the proxy.RequestHandler.
// Todo, make sure to listen for the proxy parameters from the parent by a loop.
func (proxy *Proxy) lintHandlers() error {
	proxyClient := proxy.ctx.ProxyClient()
	proxyChains, err := proxyClient.ProxyChains()
	if err != nil {
		return fmt.Errorf("proxyClient.ProxyChainsByRuleUrl: %w", err)
	}

	for i := range proxyChains {
		destination := proxyChains[i].Destination

		handlerConfigs, err := proxy.ParentManager.HandlersByRule(destination)
		if err != nil {
			return fmt.Errorf("proxy.ParentManager.HandlersByRule(rule='%v', id='%s'): %w", destination, proxy.id, err)
		}
		if len(handlerConfigs) == 0 {
			return fmt.Errorf("proxy.ParentManager.HandlersByRule(rule='%v', id='%s'): no handler configs", destination, proxy.id)
		}

		for i := range handlerConfigs {
			var h base.Interface
			if handlerConfigs[i].Type == handlerConfig.SyncReplierType {
				h = sync_replier.New()
			} else if handlerConfigs[i].Type == handlerConfig.ReplierType {
				h = replier.New()
			} else {
				return fmt.Errorf("the handler type '%s' not supported for proxy", handlerConfigs[i].Type)
			}
			// todo use the proxy category when generating a proxy id
			// it needs to over-write the generateConfig method of the parent to set a new id.
			proxy.SetHandler(handlerConfigs[i].Category, h)
		}
	}

	return nil
}

// Start the proxy
//
// Proxy can start without the parent.
// And when a parent starts, it will fetch the parameters.
// Todo make sure that proxy chain update in a live mode affects the Service.
func (proxy *Proxy) Start() (*sync.WaitGroup, error) {
	err := proxy.lintProxyChain()
	if err != nil {
		return nil, fmt.Errorf("proxy.lintProxyChain: %w", err)
	}

	// get the list of the handlers if there is no given in the handler list
	err = proxy.lintHandlers()
	if err != nil {
		return nil, fmt.Errorf("proxy.lintHandlers: %w", err)
	}

	wg, err := proxy.Auxiliary.Start()
	if err != nil {
		return nil, fmt.Errorf("proxy.Auxiliary.Start: %w", err)
	}

	return wg, nil
}
