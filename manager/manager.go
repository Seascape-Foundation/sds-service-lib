// Package manager is the manager of the service.
package manager

import (
	"fmt"
	clientConfig "github.com/ahmetson/client-lib/config"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	context "github.com/ahmetson/dev-lib"
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
	handlerManager manager_client.Interface
	logger         *log.Logger
	handlerClients []manager_client.Interface
	deps           []*clientConfig.Client
	ctx            context.Interface
}

// New service with the parameters.
// Parameter order: id, url, context type
func New(ctx context.Interface, client *clientConfig.Client) (*Manager, error) {
	handler := syncReplier.New()

	h := &Manager{
		ctx:            ctx,
		handler:        handler,
		serviceUrl:     client.ServiceUrl,
		logger:         nil,
		handlerClients: make([]manager_client.Interface, 0),
		deps:           make([]*clientConfig.Client, 0),
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
// todo It doesn't close the proxies, which it must close.
func (m *Manager) Close() error {
	// closing all handlers
	for _, client := range m.handlerClients {
		err := client.Close()
		if err != nil {
			return fmt.Errorf("handlerManagerClient('%s').Close: %v", client.Id(), err)
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

	return nil
}

// onClose received a close signal for this service
func (m *Manager) onClose(req message.Request) message.Reply {
	m.logger.Info("service received a signal to close", "service url", m.serviceUrl)

	err := m.Close()
	if err != nil {
		return req.Fail(fmt.Sprintf("manager.Close: %v", err))
	}

	m.logger.Info("service closed", "service url", m.serviceUrl)

	return req.Ok(key_value.Empty())
}

// onHeartbeat simple handler to check that service is alive
func (m *Manager) onHeartbeat(req message.Request) message.Reply {
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

func (m *Manager) SetDeps(configs []*clientConfig.Client) {
	m.deps = configs
}

// Start the orchestra in the background.
// If it failed to run, then return an error.
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
	if err := m.handler.Route("heartbeat", m.onHeartbeat); err != nil {
		return fmt.Errorf(`handler.Route("close"): %w`, err)
	}

	if err := m.handler.Start(); err != nil {
		return fmt.Errorf("handler.Start: %w", err)
	}

	return nil
}
