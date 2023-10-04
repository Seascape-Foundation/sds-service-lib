package manager

import (
	"fmt"
	"github.com/ahmetson/client-lib"
	clientConfig "github.com/ahmetson/client-lib/config"
	serviceConfig "github.com/ahmetson/config-lib/service"
	"github.com/ahmetson/datatype-lib/data_type/key_value"
	"github.com/ahmetson/datatype-lib/message"
)

//
// Interact with the service manager
//

type Client struct {
	*client.Socket
}

// NewClient returns a manager client based on the configuration
func NewClient(c *clientConfig.Client) (*Client, error) {
	socket, err := client.New(c)
	if err != nil {
		return nil, fmt.Errorf("client.New: %w", err)
	}

	return &Client{socket}, nil
}

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

// The Units method returns the destination units by a rule.
func (c *Client) Units(rule *serviceConfig.Rule) ([]*serviceConfig.Unit, error) {
	req := &message.Request{
		Command:    Units,
		Parameters: key_value.New().Set("rule", rule),
	}
	reply, err := c.Request(req)
	if err != nil {
		return nil, fmt.Errorf("c.Request: %w", err)
	}
	if !reply.IsOK() {
		return nil, fmt.Errorf("reply error message: %s", reply.ErrorMessage())
	}

	rawUnits, err := reply.ReplyParameters().NestedListValue("units")
	if err != nil {
		return nil, fmt.Errorf("reply.ReplyParameters().NestedKeyValueList('proxy_chains'): %w", err)
	}

	units := make([]*serviceConfig.Unit, len(rawUnits))
	for i, rawUnit := range rawUnits {
		var unit serviceConfig.Unit
		err = rawUnit.Interface(&unit)
		if err != nil {
			return nil, fmt.Errorf("rawUnits[%d].Interface: %w", i, err)
		}

		units[i] = &unit
	}

	return units, nil
}
