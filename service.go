// Package service is the primary service.
// This package is calling out the orchestra. Then within that orchestra sets up
// - handler manager
// - proxies
// - extensions
// - config manager
// - dep manager
package service

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	serviceConfig "github.com/ahmetson/config-lib/service"
	"github.com/ahmetson/dev-lib"
	ctxConfig "github.com/ahmetson/dev-lib/base/config"
	"github.com/ahmetson/handler-lib/base"
	handlerConfig "github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/manager_client"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/os-lib/arg"
	"github.com/ahmetson/service-lib/config"
	"github.com/ahmetson/service-lib/manager"

	"slices"
)

// Service keeps all necessary parameters of the service.
type Service struct {
	config             *serviceConfig.Service
	ctx                context.Interface // context handles the configuration and dependencies
	Handlers           key_value.KeyValue
	RequiredProxies    []string // url => orchestra type
	RequiredExtensions key_value.KeyValue
	Logger             *log.Logger
	Type               serviceConfig.Type
	id                 string
	url                string
	parentId           string
	manager            *manager.Manager // manage this service from other parts
	// as it should be called before the orchestra runs
}

// New service.
// Requires url and id.
// The url and id could be passed as flag config.IdFlag, config.UrlFlag.
// Or url and id could be passed as environment variable config.IdEnv, config.UrlEnv.
//
// It will also create the context internally.
// The created context is started.
// By default, the service uses' config.DevContext.
// It could be overwritten by a flag config.ContextFlag.
func New() (*Service, error) {
	id := ""
	url := ""
	contextType := ctxConfig.DevContext // default is used a dev context

	// let's validate the parameters of the service
	if arg.FlagExist(config.IdFlag) {
		id = arg.FlagValue(config.IdFlag)
	}
	if arg.FlagExist(config.UrlFlag) {
		url = arg.FlagValue(config.UrlFlag)
	}
	if arg.FlagExist(config.ContextFlag) {
		contextType = arg.FlagValue(config.ConfigFlag)
	}

	// Start the context
	ctx, err := context.New(contextType)
	if err != nil {
		return nil, fmt.Errorf("context.New(%s): %w", ctxConfig.DevContext, err)
	}

	err = ctx.Start()
	if err != nil {
		return nil, fmt.Errorf("ctx('%s').Start: %w", contextType, err)
	}

	if len(id) == 0 {
		configClient := ctx.Config()
		id, err = configClient.String(config.IdEnv)
		if err != nil {
			err = fmt.Errorf("configClient.String('%s'): %w", config.IdEnv, err)
			if closeErr := ctx.Config().Close(); closeErr != nil {
				return nil, fmt.Errorf("%v: ctx.Config().Close: %w", err, closeErr)
			}
			if closeErr := ctx.DepManager().Close(); closeErr != nil {
				return nil, fmt.Errorf("%v: ctx.DepManager.Close: %w", err, closeErr)

			}
			return nil, err
		}
	}
	if len(url) == 0 {
		configClient := ctx.Config()
		url, err = configClient.String(config.UrlEnv)
		if err != nil {
			err = fmt.Errorf("configClient.String('%s'): %w", config.UrlEnv, err)
			if closeErr := ctx.Config().Close(); closeErr != nil {
				return nil, fmt.Errorf("%v: ctx.Config().Close: %w", err, closeErr)
			}
			if closeErr := ctx.DepManager().Close(); closeErr != nil {
				return nil, fmt.Errorf("%v: ctx.DepManager.Close: %w", err, closeErr)

			}
			return nil, err
		}
	}

	parentId := ""
	if arg.FlagExist(config.ParentFlag) {
		parentId = arg.FlagValue(config.ParentFlag)
	}

	if len(id) == 0 {
		err = fmt.Errorf("service can not identify itself. Either use %s flag or %s environment variable", config.IdFlag, config.IdEnv)
		if closeErr := ctx.Config().Close(); closeErr != nil {
			return nil, fmt.Errorf("%v: ctx.Config().Close: %w", err, closeErr)
		}
		if closeErr := ctx.DepManager().Close(); closeErr != nil {
			return nil, fmt.Errorf("%v: ctx.DepManager.Close: %w", err, closeErr)
		}
		return nil, err
	}
	if len(url) == 0 {
		err = fmt.Errorf("service can not identify it's class. Either use %s flag or %s environment variable", config.UrlFlag, config.UrlEnv)
		if closeErr := ctx.Config().Close(); closeErr != nil {
			return nil, fmt.Errorf("%v: ctx.Config().Close: %w", err, closeErr)
		}
		if closeErr := ctx.DepManager().Close(); closeErr != nil {
			return nil, fmt.Errorf("%v: ctx.DepManager.Close: %w", err, closeErr)
		}
		return nil, err
	}

	logger, err := log.New(id, true)
	if err != nil {
		err = fmt.Errorf("log.New(%s): %w", id, err)

		if closeErr := ctx.Config().Close(); closeErr != nil {
			return nil, fmt.Errorf("%v: ctx.Config().Close: %w", err, closeErr)
		}
		if closeErr := ctx.DepManager().Close(); closeErr != nil {
			return nil, fmt.Errorf("%v: ctx.DepManager.Close: %w", err, closeErr)
		}

		return nil, err
	}

	independent := &Service{
		ctx:             ctx,
		Logger:          logger,
		Handlers:        key_value.Empty(),
		RequiredProxies: []string{},
		url:             url,
		id:              id,
		parentId:        parentId,
		Type:            serviceConfig.IndependentType,
	}

	return independent, nil
}

// SetHandler of category
func (independent *Service) SetHandler(category string, controller base.Interface) {
	independent.Handlers.Set(category, controller)
}

// SetTypeByService overwrites the type from the extended service.
// Maybe proxy or extension will do it.
func (independent *Service) SetTypeByService(newType serviceConfig.Type) {
	independent.Type = newType
}

func (independent *Service) Url() string {
	return independent.url
}

func (independent *Service) Id() string {
	return independent.id
}

// RequireProxy adds a proxy that's needed for this service to run.
// Service has to have a pipeline.
func (independent *Service) RequireProxy(url string) {
	if !independent.IsProxyRequired(url) {
		independent.RequiredProxies = append(independent.RequiredProxies, url)
	}
}

// RequireExtension lints the id to the extension url
func (independent *Service) RequireExtension(id string, url string) {
	if err := independent.RequiredExtensions.Exist(id); err == nil {
		independent.RequiredExtensions.Set(id, url)
	}
}

func (independent *Service) IsProxyRequired(proxyUrl string) bool {
	return slices.Contains(independent.RequiredProxies, proxyUrl)
}

// A Pipeline creates a chain of the proxies.
//func (independent *Service) Pipeline(pipeEnd *pipeline.PipeEnd, proxyUrls ...string) error {
//	pipelines := independent.pipelines
//	controllers := independent.Handlers
//	proxies := independent.RequiredProxies
//	createdPipeline := pipeEnd.Pipeline(proxyUrls)
//
//	if err := pipeline.PrepareAddingPipeline(pipelines, proxies, controllers, createdPipeline); err != nil {
//		return fmt.Errorf("pipeline.PrepareAddingPipeline: %w", err)
//	}
//
//	independent.pipelines = append(independent.pipelines, createdPipeline)
//
//	return nil
//}

// returns the extension urls
func (independent *Service) requiredControllerExtensions() []string {
	var extensions []string
	for _, controllerInterface := range independent.Handlers {
		c := controllerInterface.(base.Interface)
		extensions = append(extensions, c.DepIds()...)
	}

	return extensions
}

// lintPipelineConfiguration checks that proxy url and controllerName are valid.
// Then, in the config, it makes sure that dependency is linted.
//func (independent *Service) preparePipelineConfigurations() error {
//	servicePipeline := pipeline.FindServiceEnd(independent.pipelines)
//
//	if servicePipeline != nil {
//		servicePipeline.End.Url = independent.config.Url
//		independent.Logger.Info("dont forget to update the yaml with the controllerPipeline service end url")
//		err := pipeline.LintToService(independent.Context, independent.config, servicePipeline)
//		if err != nil {
//			return fmt.Errorf("pipeline.LintToService: %w", err)
//		}
//	}
//
//	err := pipeline.LintToControllers(independent.Context, independent.config, independent.pipelines)
//	if err != nil {
//		return fmt.Errorf("pipeline.LintToControllers: %w", err)
//	}
//
//	return nil
//}

// RunManager the services by validating, linting the configurations, as well as setting up the dependencies
func (independent *Service) RunManager() error {

	requiredExtensions := independent.requiredControllerExtensions()

	//
	// prepare proxies configurations
	//--------------------------------------------------
	if len(independent.RequiredProxies) > 0 {
		//for _, requiredProxy := range independent.RequiredProxies {
		//var dep *dev.Dep

		//dep, err = independent.ctx.New(requiredProxy)
		//if err != nil {
		//	err = fmt.Errorf(`service.Interface.New("%s"): %w`, requiredProxy, err)
		//	goto closeContext
		//}

		// Sets the default values.
		//if err = independent.prepareProxyConfiguration(dep); err != nil {
		//	err = fmt.Errorf("service.prepareProxyConfiguration(%s): %w", requiredProxy, err)
		//	goto closeContext
		//}
		//}

		//if len(independent.pipelines) == 0 {
		//	err = fmt.Errorf("no pipeline to lint the proxy to the handler")
		//	goto closeContext
		//}

		//if err = independent.preparePipelineConfigurations(); err != nil {
		//	err = fmt.Errorf("preparePipelineConfigurations: %w", err)
		//	goto closeContext
		//}
	}

	//
	// prepare extensions configurations
	//------------------------------------------------------
	if len(requiredExtensions) > 0 {
		independent.Logger.Warn("extensions needed to be prepared", "extensions", requiredExtensions)
		//for _, requiredExtension := range requiredExtensions {
		//var dep *dev.Dep
		//
		//dep, err = independent.Context.New(requiredExtension)
		//if err != nil {
		//	err = fmt.Errorf(`service.Interface.New("%s"): %w`, requiredExtension, err)
		//	goto closeContext
		//}
		//
		//if err = independent.prepareExtensionConfiguration(dep); err != nil {
		//	err = fmt.Errorf(`service.prepareExtensionConfiguration("%s"): %w`, requiredExtension, err)
		//	goto closeContext
		//}
		//}
	}

	var err error

	//
	// lint extensions, configurations to the controllers
	//---------------------------------------------------------
	for category, controllerInterface := range independent.Handlers {
		c := controllerInterface.(base.Interface)
		var controllerConfig *handlerConfig.Handler
		var controllerExtensions []string

		controllerConfig, err = independent.config.HandlerByCategory(category)
		if err != nil {
			err = fmt.Errorf("'%s' registered in the service, no config found: %w", category, err)
			goto closeContext
		}

		if err = c.SetLogger(independent.Logger.Child(controllerConfig.Id)); err != nil {
			err = fmt.Errorf("c.SetLogger: %w", err)
			goto closeContext
		}
		controllerExtensions = c.DepIds()
		for _, extensionUrl := range controllerExtensions {
			requiredExtension := independent.config.ExtensionByUrl(extensionUrl)
			err = c.AddDepByService(requiredExtension)
			if err != nil {
				err = fmt.Errorf("c.AddDepByService: %w", err)
				goto closeContext
			}
		}
	}

	// run proxies if they are needed.
	if len(independent.RequiredProxies) > 0 {
		//for _, requiredProxy := range independent.RequiredProxies {
		// We don't check for the error, since preparing the config should do that already.
		//dep, _ := independent.Context.Dep(requiredProxy)
		//
		//if err = independent.prepareProxy(dep); err != nil {
		//	err = fmt.Errorf(`service.prepareProxy("%s"): %w`, requiredProxy, err)
		//	goto closeContext
		//}
		//}
	}

	// run extensions if they are needed.
	if len(requiredExtensions) > 0 {
		//for _, requiredExtension := range requiredExtensions {
		// We don't check for the error, since preparing the config should do that already.
		//dep, _ := independent.Context.Dep(requiredExtension)
		//
		//if err = independent.prepareExtension(dep); err != nil {
		//	err = fmt.Errorf(`service.prepareExtension("%s"): %w`, requiredExtension, err)
		//	goto closeContext
		//}
		//}
	}

	return nil

	// error happened, close the orchestra
closeContext:
	if err == nil {
		return fmt.Errorf("error is expected, it doesn't exist though")
	}
	return err
}

func (independent *Service) generateConfig() error {
	configClient := independent.ctx.Config()

	generatedConfig, err := configClient.GenerateService(independent.id, independent.url, independent.Type)
	if err != nil {
		return fmt.Errorf("configClient.GenerateService('%s', '%s', '%s'): %w", independent.id, independent.url, independent.Type, err)
	}

	// Get all handlers and add them into the service
	for category, raw := range independent.Handlers {
		handler := raw.(base.Interface)
		generatedHandler, err := configClient.GenerateHandler(handler.Type(), category, false)
		if err != nil {
			return fmt.Errorf("configClient.GenerateHandler('%s', '%s', internal: false): %w", handler.Type(), category, err)
		}

		handler.SetConfig(generatedHandler)

		generatedConfig.SetHandler(generatedHandler)
	}

	// Some handlers were generated and added into generated service config.
	// Notify the config engine to update the service.
	if len(independent.Handlers) > 0 {
		if err := configClient.SetService(generatedConfig); err != nil {
			return fmt.Errorf("configClient.SetService('generated'): %w", err)
		}
	}

	independent.config = generatedConfig

	return nil
}

// lintConfig gets the configuration from the context and sets them in the service and handler.
func (independent *Service) lintConfig() error {
	configClient := independent.ctx.Config()

	returnedService, err := configClient.Service(independent.id)
	if err != nil {
		return fmt.Errorf("configClient.Service('%s', '%s', '%s'): %w", independent.id, independent.url, independent.Type, err)
	}

	if returnedService.Url != independent.url {
		independent.url = returnedService.Url
	}
	if returnedService.Type != independent.Type {
		independent.Type = returnedService.Type
	}

	for category, raw := range independent.Handlers {
		handler := raw.(base.Interface)

		returnedHandler, err := returnedService.HandlerByCategory(category)
		if err != nil {
			generatedHandler, err := configClient.GenerateHandler(handler.Type(), category, false)
			if err != nil {
				return fmt.Errorf("configClient.GenerateHandler('%s', '%s', internal: false): %w", handler.Type(), category, err)
			}

			handler.SetConfig(generatedHandler)

			returnedService.SetHandler(generatedHandler)
			if err := configClient.SetService(returnedService); err != nil {
				return fmt.Errorf("configClient.SetService('returned'): %w", err)
			}
		} else {
			handler.SetConfig(returnedHandler)
		}
	}

	independent.config = returnedService

	return nil
}

// prepareConfig sets the configuration of this service and handlers.
// if the configuration doesn't exist, generates the service and handler.
// the returned configuration from the context is linted into service and handler.
func (independent *Service) prepareConfig() error {
	configClient := independent.ctx.Config()

	// prepare the configuration
	exist, err := configClient.ServiceExist(independent.id)
	if err != nil {
		return fmt.Errorf("configClient.ServiceExist('%s'): %w", independent.id, err)
	}

	if !exist {
		if err = independent.generateConfig(); err != nil {
			return fmt.Errorf("generateConfig: %w", err)
		}
		return nil
	}

	if err = independent.lintConfig(); err != nil {
		return fmt.Errorf("lintConfig: %w", err)
	}

	return nil
}

// newManager creates a manager of the service.
func (independent *Service) newManager() error {
	if independent.config == nil {
		return fmt.Errorf("independent.config is nill")
	}
	m := manager.New(independent.config.Manager)
	err := m.SetLogger(independent.Logger)
	if err != nil {
		return fmt.Errorf("manager.SetLogger: %w", err)
	}
	independent.manager = m

	independent.manager.SetDepClient(independent.ctx.DepManager())
	return nil
}

// setHandlerClient creates a handler manager clients and sets them into the service manager.
func (independent *Service) setHandlerClient(c base.Interface) error {
	handlerClient, err := manager_client.New(c.Config())
	if err != nil {
		return fmt.Errorf("manager_client.New('%s'): %w", c.Config().Category, err)
	}
	independent.manager.SetHandlerClients([]manager_client.Interface{handlerClient})

	return nil
}

// startHandler sets the log into the handler which is prepared already.
// then, starts it.
func (independent *Service) startHandler(handler base.Interface) error {
	if err := handler.SetLogger(independent.Logger.Child(handler.Config().Id)); err != nil {
		return fmt.Errorf("handler(id: '%s').SetLogger: %w", handler.Config().Id, err)
	}

	if err := handler.Start(); err != nil {
		return fmt.Errorf("handler(category: '%s').Start: %w", handler.Config().Category, err)
	}

	return nil
}

// Run the service.
func (independent *Service) Run() error {
	if len(independent.Handlers) == 0 {
		return fmt.Errorf("no Handlers. call service.SetHandler")
	}

	if err := independent.prepareConfig(); err != nil {
		return fmt.Errorf("prepareConfig: %w", err)
	}

	if err := independent.newManager(); err != nil {
		return fmt.Errorf("newManager: %w", err)
	}

	var err error

	for category, raw := range independent.Handlers {
		handler := raw.(base.Interface)
		if err = independent.setHandlerClient(handler); err != nil {
			err = fmt.Errorf("manager_client.New('%s'): %w", category, err)
			goto errOccurred
		}

		if err = independent.startHandler(handler); err != nil {
			err = fmt.Errorf("startHandler: %w", err)
			goto errOccurred
		}
	}

	// todo
	// prepare the proxies by calling them in the context.
	// prepare the proxies by setting them into the independent.manager.
	// prepare the extensions by calling them in the context.
	// prepare the extensions by setting them into the independent.manager.

	err = independent.manager.Start()
	if err != nil {
		err = fmt.Errorf("service.manager.Start: %w", err)
		goto errOccurred
	}

	//err = independent.Context.ServiceReady(independent.Logger)
	//if err != nil {
	//	goto errOccurred
	//}

errOccurred:
	if err != nil {
		//if independent.Context != nil {
		//	independent.Logger.Warn("orchestra wasn't closed, close it")
		//	independent.Logger.Warn("might happen a race condition." +
		//		"if the error occurred in the handler" +
		//		"here we will close the orchestra." +
		//		"orchestra will close the service." +
		//		"service will again will come to this place, since all controllers will be cleaned out" +
		//		"and handler empty will come to here, it will try to close orchestra again",
		//	)
		//	closeErr := independent.Context.Close(independent.Logger)
		//	if closeErr != nil {
		//		independent.Logger.Fatal("service.Interface.Close", "error", closeErr, "error to print", err)
		//	}
		//}

		return err
	}
	return nil
}

//
//// prepareProxy links the proxy with the dependency.
////
//// if dependency doesn't exist, it will be downloaded
//func (independent *Service) prepareProxy(dep *dev.Dep) error {
//	// todo find the proxy url by it's id in the services list.
//	// the config.Proxy() accepts id not a url.
//	proxyConfiguration := independent.config.Proxy(dep.Url())
//
//	independent.Logger.Info("prepare proxy", "id", proxyConfiguration.Id)
//	//err := dep.Run(proxyConfiguration.Instances[0].Port, independent.Logger)
//	//if err != nil {
//	//	return fmt.Errorf(`dep.Run("%s"): %w`, dep.Url(), err)
//	//}
//
//	return nil
//}
//
//// prepareExtension links the extension with the dependency.
////
//// if dependency doesn't exist, it will be downloaded
//func (independent *Service) prepareExtension(dep *dev.Dep) error {
//	extensionConfiguration := independent.config.ExtensionByUrl(dep.Url())
//
//	independent.Logger.Info("prepare extension", "url", extensionConfiguration.Url, "port", extensionConfiguration.Port)
//	err := dep.Run(extensionConfiguration.Port, independent.Logger)
//	if err != nil {
//		return fmt.Errorf(`dep.Run("%s"): %w`, dep.Url(), err)
//	}
//	return nil
//}
//
//// prepareProxyConfiguration links the proxy with the dependency.
////
//// if dependency doesn't exist, it will be downloaded
//func (independent *Service) prepareProxyConfiguration(dep *dev.Dep) error {
//	err := dep.Prepare(independent.Logger)
//	if err != nil {
//		return fmt.Errorf("dev.Prepare(%s): %w", dep.Url(), err)
//	}
//
//	err = dep.PrepareConfig(independent.Logger)
//	if err != nil {
//		return fmt.Errorf("dev.PrepareConfig(%s): %w", dep.Url(), err)
//	}
//
//	//depConfig, err := dep.GetServiceConfig()
//	//converted, err := converter.ServiceToProxy(depConfig)
//	//if err != nil {
//	//	return fmt.Errorf("config.ServiceToProxy: %w", err)
//	//}
//
//	//proxyConfiguration := independent.config.GetProxy(dep.Url())
//	//if proxyConfiguration == nil {
//	//	independent.config.SetProxy(&converted)
//	//} else {
//	//	if strings.Compare(proxyConfiguration.Url, converted.Url) != 0 {
//	//		return fmt.Errorf("the proxy urls are not matching. in your config: %s, in the deps: %s", proxyConfiguration.Url, converted.Url)
//	//	}
//	//	if proxyConfiguration.Instances[0].Port != converted.Instances[0].Port {
//	//		independent.Logger.Warn("dependency port not matches to the proxy port. Overwriting the source", "port", proxyConfiguration.Instances[0].Port, "dependency port", converted.Instances[0].Port)
//	//
//	//		source, _ := depConfig.GetController(service.SourceName)
//	//		source.Instances[0].Port = proxyConfiguration.Instances[0].Port
//	//
//	//depConfig.SetController(source)
//	//
//	//err = dep.SetServiceConfig(depConfig)
//	//if err != nil {
//	//	return fmt.Errorf("failed to update source port in dependency proxy: '%s': %w", dep.Url(), err)
//	//}
//	//}
//	//}
//
//	return nil
//}

//func (independent *Service) prepareExtensionConfiguration(dep *dev.Dep) error {
//	err := dep.Prepare(independent.Logger)
//	if err != nil {
//		return fmt.Errorf("dev.Prepare(%s): %w", dep.Url(), err)
//	}
//
//	err = dep.PrepareConfig(independent.Logger)
//	if err != nil {
//		return fmt.Errorf("dev.PrepareConfig on %s: %w", dep.Url(), err)
//	}
//
//	//depConfig, err := dep.GetServiceConfig()
//	//converted, err := converter.ServiceToExtension(depConfig)
//	//if err != nil {
//	//	return fmt.Errorf("config.ServiceToExtension: %w", err)
//	//}
//	//
//	//extensionConfiguration := independent.config.GetExtension(dep.Url())
//	//if extensionConfiguration == nil {
//	//	independent.config.SetExtension(&converted)
//	//} else {
//	//	if strings.Compare(extensionConfiguration.Url, converted.Url) != 0 {
//	//		return fmt.Errorf("the extension url in your '%s' config not matches to '%s' in the dependency", extensionConfiguration.Url, converted.Url)
//	//	}
//	//	if extensionConfiguration.Port != converted.Port {
//	//		independent.Logger.Warn("dependency port not matches to the extension port. Overwriting the source", "port", extensionConfiguration.Port, "dependency port", converted.Port)
//	//
//	//		main, _ := depConfig.GetFirstController()
//	//		main.Instances[0].Port = extensionConfiguration.Port
//	//
//	//depConfig.SetController(main)
//	//
//	//err = dep.SetServiceConfig(depConfig)
//	//if err != nil {
//	//	return fmt.Errorf("failed to update port in dependency extension: '%s': %w", dep.Url(), err)
//	//}
//	//}
//	//}
//
//	return nil
//}
