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
	clientConfig "github.com/ahmetson/client-lib/config"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/dev-lib"
	ctxConfig "github.com/ahmetson/dev-lib/base/config"
	"github.com/ahmetson/handler-lib/base"
	handlerConfig "github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/manager_client"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/os-lib/arg"
	"github.com/ahmetson/service-lib/config"
	"github.com/ahmetson/service-lib/config/service"
	"github.com/ahmetson/service-lib/config/service/converter"
	"github.com/ahmetson/service-lib/config/service/pipeline"
	"github.com/ahmetson/service-lib/manager"
	"github.com/ahmetson/service-lib/orchestra/dev"
	"slices"
	"strings"
	"sync"
)

// Service keeps all necessary parameters of the service.
type Service struct {
	Config             *config.Service
	ctx                context.Interface // context handles the configuration and dependencies
	Handlers           key_value.KeyValue
	pipelines          []*pipeline.Pipeline // Pipeline beginning: url => [Pipes]
	RequiredProxies    []string             // url => orchestra type
	RequiredExtensions key_value.KeyValue
	Logger             *log.Logger
	Context            *dev.Context
	id                 string
	url                string
	parentUrl          string
	manager            *manager.Manager // manage this service from other parts
	// as it should be called before the orchestra runs
}

// New service with the parameters.
// Parameter order: id, url, context type
func New(params ...string) (*Service, error) {
	id := ""
	if len(params) > 0 {
		id = params[0]
	}
	url := ""
	if len(params) > 1 {
		url = params[1]
	}
	contextType := ctxConfig.DevContext
	if len(params) > 2 {
		contextType = params[2]
	}
	logger, err := log.New(id, true)
	if err != nil {
		return nil, fmt.Errorf("log.New(%s): %w", id, err)
	}

	ctx, err := context.New(contextType)
	if err != nil {
		return nil, fmt.Errorf("context.New(%s): %w", ctxConfig.DevContext, err)
	}

	// let's validate the parameters of the service
	if arg.FlagExist(config.IdFlag) {
		id = arg.FlagValue(config.IdFlag)
	}
	if arg.FlagExist(config.UrlFlag) {
		url = arg.FlagValue(config.UrlFlag)
	}

	parentUrl := ""
	if arg.FlagExist(config.ParentFlag) {
		parentUrl = arg.FlagValue(config.ParentFlag)
	}

	m := manager.New(id, url)
	err = m.SetLogger(logger)
	if err != nil {
		return nil, fmt.Errorf("manager.SetLogger: %w", err)
	}

	independent := &Service{
		ctx:             ctx,
		Logger:          logger,
		Handlers:        key_value.Empty(),
		RequiredProxies: []string{},
		pipelines:       make([]*pipeline.Pipeline, 0),
		url:             url,
		id:              id,
		parentUrl:       parentUrl,
	}

	return independent, nil
}

// SetHandler of category
func (independent *Service) SetHandler(id string, controller base.Interface) {
	independent.Handlers.Set(id, controller)
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
func (independent *Service) Pipeline(pipeEnd *pipeline.PipeEnd, proxyUrls ...string) error {
	pipelines := independent.pipelines
	controllers := independent.Handlers
	proxies := independent.RequiredProxies
	createdPipeline := pipeEnd.Pipeline(proxyUrls)

	if err := pipeline.PrepareAddingPipeline(pipelines, proxies, controllers, createdPipeline); err != nil {
		return fmt.Errorf("pipeline.PrepareAddingPipeline: %w", err)
	}

	independent.pipelines = append(independent.pipelines, createdPipeline)

	return nil
}

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
// Then, in the Config, it makes sure that dependency is linted.
func (independent *Service) preparePipelineConfigurations() error {
	servicePipeline := pipeline.FindServiceEnd(independent.pipelines)

	if servicePipeline != nil {
		servicePipeline.End.Url = independent.Config.Url
		independent.Logger.Info("dont forget to update the yaml with the controllerPipeline service end url")
		err := pipeline.LintToService(independent.Context, independent.Config, servicePipeline)
		if err != nil {
			return fmt.Errorf("pipeline.LintToService: %w", err)
		}
	}

	err := pipeline.LintToControllers(independent.Context, independent.Config, independent.pipelines)
	if err != nil {
		return fmt.Errorf("pipeline.LintToControllers: %w", err)
	}

	return nil
}

// RunManager the services by validating, linting the configurations, as well as setting up the dependencies
func (independent *Service) RunManager() error {
	if len(independent.Handlers) == 0 {
		return fmt.Errorf("no Handlers. call service.SetHandler")
	}

	requiredExtensions := independent.requiredControllerExtensions()

	//
	// prepare the orchestra for dependencies
	//---------------------------------------------------
	err := independent.Context.Run(independent.Logger)
	if err != nil {
		return fmt.Errorf("orchestra.Run: %w", err)
	}

	//
	// prepare proxies configurations
	//--------------------------------------------------
	if len(independent.RequiredProxies) > 0 {
		for _, requiredProxy := range independent.RequiredProxies {
			var dep *dev.Dep

			dep, err = independent.Context.New(requiredProxy)
			if err != nil {
				err = fmt.Errorf(`service.Interface.New("%s"): %w`, requiredProxy, err)
				goto closeContext
			}

			// Sets the default values.
			if err = independent.prepareProxyConfiguration(dep); err != nil {
				err = fmt.Errorf("service.prepareProxyConfiguration(%s): %w", requiredProxy, err)
				goto closeContext
			}
		}

		if len(independent.pipelines) == 0 {
			err = fmt.Errorf("no pipepline to lint the proxy to the handler")
			goto closeContext
		}

		if err = independent.preparePipelineConfigurations(); err != nil {
			err = fmt.Errorf("preparePipelineConfigurations: %w", err)
			goto closeContext
		}
	}

	//
	// prepare extensions configurations
	//------------------------------------------------------
	if len(requiredExtensions) > 0 {
		independent.Logger.Warn("extensions needed to be prepared", "extensions", requiredExtensions)
		for _, requiredExtension := range requiredExtensions {
			var dep *dev.Dep

			dep, err = independent.Context.New(requiredExtension)
			if err != nil {
				err = fmt.Errorf(`service.Interface.New("%s"): %w`, requiredExtension, err)
				goto closeContext
			}

			if err = independent.prepareExtensionConfiguration(dep); err != nil {
				err = fmt.Errorf(`service.prepareExtensionConfiguration("%s"): %w`, requiredExtension, err)
				goto closeContext
			}
		}
	}

	//
	// lint extensions, configurations to the controllers
	//---------------------------------------------------------
	for name, controllerInterface := range independent.Handlers {
		c := controllerInterface.(base.Interface)
		var controllerConfig *handlerConfig.Handler
		var controllerExtensions []string

		controllerConfig, err = independent.Config.GetController(name)
		if err != nil {
			err = fmt.Errorf("c '%s' registered in the service, no config found: %w", name, err)
			goto closeContext
		}

		c.SetConfig(controllerConfig)
		hClient, err := manager_client.New(controllerConfig)
		if err != nil {
			err = fmt.Errorf("manager_client.New: %w", err)
			goto closeContext
		}
		independent.manager.SetHandlerClients([]manager_client.Interface{hClient})

		if err = c.SetLogger(independent.Logger.Child(name)); err != nil {
			err = fmt.Errorf("c.SetLogger: %w", err)
			goto closeContext
		}
		controllerExtensions = c.DepIds()
		for _, extensionUrl := range controllerExtensions {
			requiredExtension := independent.Config.GetExtension(extensionUrl)
			req := &clientConfig.Client{
				ServiceUrl: requiredExtension.Url,
				Id:         requiredExtension.Id,
				Port:       requiredExtension.Port,
			}
			err = c.AddDepByService(req)
			if err != nil {
				err = fmt.Errorf("c.AddDepByService: %w", err)
				goto closeContext
			}
		}
	}

	// run proxies if they are needed.
	if len(independent.RequiredProxies) > 0 {
		for _, requiredProxy := range independent.RequiredProxies {
			// We don't check for the error, since preparing the config should do that already.
			dep, _ := independent.Context.Dep(requiredProxy)

			if err = independent.prepareProxy(dep); err != nil {
				err = fmt.Errorf(`service.prepareProxy("%s"): %w`, requiredProxy, err)
				goto closeContext
			}
		}
	}

	// run extensions if they are needed.
	if len(requiredExtensions) > 0 {
		for _, requiredExtension := range requiredExtensions {
			// We don't check for the error, since preparing the config should do that already.
			dep, _ := independent.Context.Dep(requiredExtension)

			if err = independent.prepareExtension(dep); err != nil {
				err = fmt.Errorf(`service.prepareExtension("%s"): %w`, requiredExtension, err)
				goto closeContext
			}
		}
	}

	return nil

	// error happened, close the orchestra
closeContext:
	if err == nil {
		return fmt.Errorf("error is expected, it doesn't exist though")
	}
	return err
}

// Run the service.
func (independent *Service) Run() error {
	var wg sync.WaitGroup

	err := independent.RunManager()
	if err != nil {
		goto errOccurred
	}

	err = independent.ctx.Start()
	if err != nil {
		goto errOccurred
	}

	independent.manager.SetDepClient(independent.ctx.DepManager())

	for id, controllerInterface := range independent.Handlers {
		c := controllerInterface.(base.Interface)

		err = c.Start()
		if err != nil {
			err = fmt.Errorf("handler('%s').Start: %w", id, err)
			goto errOccurred
		}
	}

	err = independent.manager.Start()
	if err != nil {
		err = fmt.Errorf("service.manager.Start: %w", err)
		goto errOccurred
	}

	err = independent.Context.ServiceReady(independent.Logger)
	if err != nil {
		goto errOccurred
	}

	wg.Wait()

errOccurred:
	if err != nil {
		if independent.Context != nil {
			independent.Logger.Warn("orchestra wasn't closed, close it")
			independent.Logger.Warn("might happen a race condition." +
				"if the error occurred in the handler" +
				"here we will close the orchestra." +
				"orchestra will close the service." +
				"service will again will come to this place, since all controllers will be cleaned out" +
				"and handler empty will come to here, it will try to close orchestra again",
			)
			closeErr := independent.Context.Close(independent.Logger)
			if closeErr != nil {
				independent.Logger.Fatal("service.Interface.Close", "error", closeErr, "error to print", err)
			}
		}

		return err
	}
	return nil
}

// prepareProxy links the proxy with the dependency.
//
// if dependency doesn't exist, it will be downloaded
func (independent *Service) prepareProxy(dep *dev.Dep) error {
	proxyConfiguration := independent.Config.GetProxy(dep.Url())

	independent.Logger.Info("prepare proxy", "url", proxyConfiguration.Url, "port", proxyConfiguration.Instances[0].Port)
	err := dep.Run(proxyConfiguration.Instances[0].Port, independent.Logger)
	if err != nil {
		return fmt.Errorf(`dep.Run("%s"): %w`, dep.Url(), err)
	}

	return nil
}

// prepareExtension links the extension with the dependency.
//
// if dependency doesn't exist, it will be downloaded
func (independent *Service) prepareExtension(dep *dev.Dep) error {
	extensionConfiguration := independent.Config.GetExtension(dep.Url())

	independent.Logger.Info("prepare extension", "url", extensionConfiguration.Url, "port", extensionConfiguration.Port)
	err := dep.Run(extensionConfiguration.Port, independent.Logger)
	if err != nil {
		return fmt.Errorf(`dep.Run("%s"): %w`, dep.Url(), err)
	}
	return nil
}

// prepareProxyConfiguration links the proxy with the dependency.
//
// if dependency doesn't exist, it will be downloaded
func (independent *Service) prepareProxyConfiguration(dep *dev.Dep) error {
	err := dep.Prepare(independent.Logger)
	if err != nil {
		return fmt.Errorf("dev.Prepare(%s): %w", dep.Url(), err)
	}

	err = dep.PrepareConfig(independent.Logger)
	if err != nil {
		return fmt.Errorf("dev.PrepareConfig(%s): %w", dep.Url(), err)
	}

	depConfig, err := dep.GetServiceConfig()
	converted, err := converter.ServiceToProxy(depConfig)
	if err != nil {
		return fmt.Errorf("config.ServiceToProxy: %w", err)
	}

	proxyConfiguration := independent.Config.GetProxy(dep.Url())
	if proxyConfiguration == nil {
		independent.Config.SetProxy(&converted)
	} else {
		if strings.Compare(proxyConfiguration.Url, converted.Url) != 0 {
			return fmt.Errorf("the proxy urls are not matching. in your config: %s, in the deps: %s", proxyConfiguration.Url, converted.Url)
		}
		if proxyConfiguration.Instances[0].Port != converted.Instances[0].Port {
			independent.Logger.Warn("dependency port not matches to the proxy port. Overwriting the source", "port", proxyConfiguration.Instances[0].Port, "dependency port", converted.Instances[0].Port)

			source, _ := depConfig.GetController(service.SourceName)
			//source.Instances[0].Port = proxyConfiguration.Instances[0].Port

			depConfig.SetController(source)

			err = dep.SetServiceConfig(depConfig)
			if err != nil {
				return fmt.Errorf("failed to update source port in dependency porxy: '%s': %w", dep.Url(), err)
			}
		}
	}

	return nil
}

func (independent *Service) prepareExtensionConfiguration(dep *dev.Dep) error {
	err := dep.Prepare(independent.Logger)
	if err != nil {
		return fmt.Errorf("dev.Prepare(%s): %w", dep.Url(), err)
	}

	err = dep.PrepareConfig(independent.Logger)
	if err != nil {
		return fmt.Errorf("dev.PrepareConfig on %s: %w", dep.Url(), err)
	}

	depConfig, err := dep.GetServiceConfig()
	converted, err := converter.ServiceToExtension(depConfig)
	if err != nil {
		return fmt.Errorf("config.ServiceToExtension: %w", err)
	}

	extensionConfiguration := independent.Config.GetExtension(dep.Url())
	if extensionConfiguration == nil {
		independent.Config.SetExtension(&converted)
	} else {
		if strings.Compare(extensionConfiguration.Url, converted.Url) != 0 {
			return fmt.Errorf("the extension url in your '%s' config not matches to '%s' in the dependency", extensionConfiguration.Url, converted.Url)
		}
		if extensionConfiguration.Port != converted.Port {
			independent.Logger.Warn("dependency port not matches to the extension port. Overwriting the source", "port", extensionConfiguration.Port, "dependency port", converted.Port)

			main, _ := depConfig.GetFirstController()
			//main.Instances[0].Port = extensionConfiguration.Port

			depConfig.SetController(main)

			err = dep.SetServiceConfig(depConfig)
			if err != nil {
				return fmt.Errorf("failed to update port in dependency extension: '%s': %w", dep.Url(), err)
			}
		}
	}

	return nil
}
