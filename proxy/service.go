// Package proxy defines the script that acts as the middleware
package proxy

import (
	"fmt"
	"github.com/ahmetson/service-lib/config"
	service2 "github.com/ahmetson/service-lib/config/service"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/server"
	"sync"
)

type service = service.Service

// Proxy defines the parameters of the proxy service
type Proxy struct {
	*service
	// Controller that handles the requests and redirects to the destination.
	Controller *Controller
}

// An extension creates the config of the proxy server.
// The proxy server itself is added as the extension to the source controllers,
// to the request handlers and to the reply handlers.
func extension() *service2.Extension {
	return service2.NewInternalExtension(ControllerName)
}

// registerDestination registers the server instances as the destination.
// It adds the server config.
func (proxy *Proxy) registerDestination() {
	for _, c := range proxy.service.Config.Service.Controllers {
		if c.Category == service2.DestinationName {
			proxy.Controller.RegisterDestination(c, proxy.service.Config.Service.Url)
			break
		}
	}
}

// New proxy service along with its server.
func New(config *config.Config, parent *log.Logger) *Proxy {
	logger := parent.Child("service", "service_type", service2.ProxyType)

	base, _ := service.New(config, logger)

	service := Proxy{
		service:    base,
		Controller: newController(logger.Child("server")),
	}

	return &service
}

func (proxy *Proxy) getSource() server.Interface {
	controllers := proxy.service.Controllers.Map()
	source := controllers[service2.SourceName].(server.Interface)
	return source
}

func (proxy *Proxy) Prepare() error {
	if proxy.Controller.requiredDestination == service2.UnknownType {
		return fmt.Errorf("missing the required destination. call proxy.ControllerCategory.RequireDestination")
	}

	if err := proxy.service.Prepare(service2.ProxyType); err != nil {
		return fmt.Errorf("service.Run as '%s' failed: %w", service2.ProxyType, err)
	}

	if err := proxy.service.PrepareControllerConfiguration(service2.DestinationName, proxy.Controller.requiredDestination); err != nil {
		return fmt.Errorf("prepare destination as '%s' failed: %w", proxy.Controller.requiredDestination, err)
	}

	proxy.registerDestination()

	return nil
}

// SetDefaultSource creates a source server of the given type.
//
// It loads the source name automatically.
func (proxy *Proxy) SetDefaultSource(controllerType service2.ControllerType) error {
	// todo move the validation to the proxy.ValidateTypes() function
	var source server.Interface
	if controllerType == service2.SyncReplierType {
		sourceController, err := server.SyncReplier(proxy.service.Logger)
		if err != nil {
			return fmt.Errorf("failed to create a source as server.NewReplier: %w", err)
		}
		source = sourceController
	} else if controllerType == service2.PusherType {
		sourceController, err := server.NewPull(proxy.service.Logger)
		if err != nil {
			return fmt.Errorf("failed to create a source as server.NewPull: %w", err)
		}
		source = sourceController
	} else {
		return fmt.Errorf("the '%s' server type not supported", controllerType)
	}

	proxy.SetCustomSource(source)

	return nil
}

// SetCustomSource sets the source server, and invokes the source server's
func (proxy *Proxy) SetCustomSource(source server.Interface) {
	proxy.service.AddController(service2.SourceName, source)
}

// Run the proxy service.
func (proxy *Proxy) Run() {
	// call BuildConfiguration explicitly to generate the yaml without proxy server's extension.
	proxy.service.BuildConfiguration()

	// we add the proxy extension to the source.
	// source can forward messages along with a route.
	proxyExtension := extension()

	// The proxy adds itself as the extension to the sources
	// after validation of the previous extensions
	proxy.getSource().RequireExtension(proxyExtension.Url)
	proxy.getSource().AddExtensionConfig(proxyExtension)
	go proxy.service.Run()

	// Run the proxy server. Proxy server itself on the other hand
	// will run the destination clients
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		proxy.Controller.Run()
		wg.Done()
	}()

	wg.Wait()
}
