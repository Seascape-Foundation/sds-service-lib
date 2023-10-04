package service

import (
	"fmt"
	clientConfig "github.com/ahmetson/client-lib/config"
	"github.com/ahmetson/datatype-lib/data_type/key_value"
	"github.com/ahmetson/os-lib/arg"
	"github.com/ahmetson/service-lib/flag"
	"github.com/ahmetson/service-lib/manager"
)

type Auxiliary struct {
	*Service
	ParentManager *manager.Client // parent to work with
}

// NewAuxiliary creates a service with the parent.
func NewAuxiliary() (*Auxiliary, error) {
	if !arg.FlagExist(flag.ParentFlag) {
		return nil, fmt.Errorf("missing %s flag", arg.NewFlag(flag.ParentFlag))
	}

	//
	// Parent config in a raw string format
	//
	parentStr := arg.FlagValue(flag.ParentFlag)
	parentKv, err := key_value.NewFromString(parentStr)
	if err != nil {
		return nil, fmt.Errorf("key_value.NewFromString('%s'): %w", flag.ParentFlag, err)
	}

	//
	// Parent config
	//
	var parentConfig clientConfig.Client
	err = parentKv.Interface(&parentConfig)
	if err != nil {
		return nil, fmt.Errorf("parentKv.Interface: %w", err)
	}
	if len(parentConfig.Id) == 0 {
		return nil, fmt.Errorf("empty parent")
	}
	parentConfig.UrlFunc(clientConfig.Url)

	//
	// Parent client
	//
	parent, err := manager.NewClient(&parentConfig)
	if err != nil {
		return nil, fmt.Errorf("manager.NewClient('parentConfig'): %w", err)
	}

	independent, err := New()
	if err != nil {
		return nil, fmt.Errorf("new independent service: %w", err)
	}

	return &Auxiliary{Service: independent, ParentManager: parent}, nil
}

// setProxyUnits prepares the proxy chains by fetching the proxies from the parent
// and storing them in this service
func (auxiliary *Auxiliary) setProxyUnits() error {
	parentClient := auxiliary.ParentManager
	proxyChains, err := parentClient.ProxyChainsByLastProxy(auxiliary.id)
	if err != nil {
		return fmt.Errorf("auxiliary.ParentManager.ProxyChainsByRuleUrl: %w", err)
	}

	proxyClient := auxiliary.ctx.ProxyClient()

	auxiliary.Logger.Warn("copying proxy chain rule from the parent to the child",
		"warning 1", "the proxy may be over-writing it by adding another units",
		"solution 1", "to the Set and SetUnits of proxy client add an merge flag so it will add to already existing data")

	// set the proxy destination units for each rule
	for _, proxyChain := range proxyChains {
		// the last proxy in the list is removed as its this service
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
