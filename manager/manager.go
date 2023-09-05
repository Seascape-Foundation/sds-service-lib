// Package manager is the manager of the service.
package manager

import (
	"fmt"
	clientConfig "github.com/ahmetson/client-lib/config"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/dev-lib/dep_client"
	"github.com/ahmetson/handler-lib/base"
	handlerConfig "github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/manager_client"
	syncReplier "github.com/ahmetson/handler-lib/sync_replier"
	"github.com/ahmetson/log-lib"
)

// The Manager keeps all necessary parameters of the service.
type Manager struct {
	serviceUrl     string
	handler        base.Interface // manage this service from other parts. it should be called before the orchestra runs
	logger         *log.Logger
	handlerClients []manager_client.Interface
	depClient      dep_client.Interface
	deps           []*clientConfig.Client
}

// New service with the parameters.
// Parameter order: id, url, context type
func New(client *clientConfig.Client) *Manager {
	handler := syncReplier.New()

	h := &Manager{
		handler:    handler,
		serviceUrl: client.ServiceUrl,
	}

	managerConfig := h.Config(client)
	handler.SetConfig(managerConfig)

	return h
}

// onClose closing all the dependencies in the orchestra as well as all handlers
func (m *Manager) onClose(req message.Request) message.Reply {
	m.logger.Info("service received a signal to close", "service url", m.serviceUrl)

	// closing all handlers
	for _, client := range m.handlerClients {
		parts, _, err := client.Parts()
		if err != nil {
			return req.Fail(fmt.Sprintf("client.Parts: %v", err))
		}
		for _, part := range parts {
			// I expect that the killing process will release its resources as well.
			err := client.ClosePart(part)
			if err != nil {
				return req.Fail(fmt.Sprintf(`handler.ClosePart("%s"): %v`, part, err))
			}
		}
	}

	// closing all dependencies
	for _, c := range m.deps {
		if err := m.depClient.CloseDep(c); err != nil {
			return req.Fail(fmt.Sprintf("depClient.CloseDep('%s'): %v", c.Id, err))
		}
	}

	return req.Ok(key_value.Empty())
}

// Config of the manager
func (m *Manager) Config(client *clientConfig.Client) *handlerConfig.Handler {
	return &handlerConfig.Handler{
		Type:           handlerConfig.SyncReplierType,
		Category:       "service",
		InstanceAmount: 1,
		Port:           client.Port,
		Id:             client.Id,
	}
}

func (m *Manager) SetLogger(parent *log.Logger) error {
	logger := parent.Child("manager")
	m.logger = logger

	if err := m.handler.SetLogger(logger); err != nil {
		return fmt.Errorf("handler.SetLogger: %w", err)
	}

	return nil
}

func (m *Manager) SetHandlerClients(clients []manager_client.Interface) {
	m.handlerClients = append(m.handlerClients, clients...)
}

func (m *Manager) SetDepClient(client dep_client.Interface) {
	m.depClient = client
}

func (m *Manager) SetDeps(configs []*clientConfig.Client) {
	m.deps = configs
}

// Start the orchestra in the background. If it failed to run, then return an error.
// The url request is the main service to which this orchestra belongs too.
//
// The logger is the handler logger as it is. The orchestra will create its own logger from it.
func (m *Manager) Start() error {
	if m.logger == nil {
		return fmt.Errorf("logger not set. call SetLogger first")
	}
	if err := m.handler.Route("close", m.onClose); err != nil {
		return fmt.Errorf(`handler.Route("close"): %w`, err)
	}

	if err := m.handler.Start(); err != nil {
		return fmt.Errorf("handler.Start: %w", err)
	}

	return nil
}
