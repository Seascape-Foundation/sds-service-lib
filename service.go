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
	"github.com/ahmetson/client-lib"
	clientConfig "github.com/ahmetson/client-lib/config"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	config2 "github.com/ahmetson/config-lib"
	"github.com/ahmetson/dev-lib"
	ctxConfig "github.com/ahmetson/dev-lib/config"
	"github.com/ahmetson/handler-lib"
	"github.com/ahmetson/handler-lib/command"
	handlerConfig "github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/os-lib/arg"
	"github.com/ahmetson/service-lib/config"
	"github.com/ahmetson/service-lib/config/service"
	"github.com/ahmetson/service-lib/config/service/converter"
	"github.com/ahmetson/service-lib/config/service/pipeline"
	"github.com/ahmetson/service-lib/orchestra/dev"
	"slices"
	"strings"
	"sync"
)

// Service keeps all necessary parameters of the service.
type Service struct {
	Config          *config.Service
	ctx             context.Interface
	Controllers     key_value.KeyValue
	pipelines       []*pipeline.Pipeline // Pipeline beginning: url => [Pipes]
	RequiredProxies []string             // url => orchestra type
	Logger          *log.Logger
	Context         *dev.Context
	id              string
	url             string
	parentUrl       string
	manager         *handler.Controller // manage this service from other parts. it should be called before the orchestra runs
}

func New(id string) (*Service, error) {
	logger, err := log.New(id, true)
	if err != nil {
		return nil, fmt.Errorf("log.New(%s): %w", id, err)
	}

	ctx, err := context.New(ctxConfig.DevContext)
	if err != nil {
		return nil, fmt.Errorf("context.New(%s): %w", ctxConfig.DevContext, err)
	}

	// let's validate the parameters of the service
	if arg.Exist(config.IdFlag) {
		id, err = arg.Value(config.IdFlag)
		if err != nil {
			return nil, fmt.Errorf("arg.Value(--%s): %w", config.IdFlag, err)
		}
	}

	url := ""
	if arg.Exist(config.UrlFlag) {
		url, err = arg.Value(config.UrlFlag)
		if err != nil {
			return nil, fmt.Errorf("arg.Value(--%s): %w", config.UrlFlag, err)
		}
	}

	parentUrl := ""
	if arg.Exist(config.ParentFlag) {
		parentUrl, err = arg.Value(config.ParentFlag)
		if err != nil {
			return nil, fmt.Errorf("arg.Value(--%s): %w", config.ParentFlag, err)
		}
	}

	independent := &Service{
		ctx:             ctx,
		Logger:          logger,
		Controllers:     key_value.Empty(),
		RequiredProxies: []string{},
		pipelines:       make([]*pipeline.Pipeline, 0),
		url:             url,
		id:              id,
		parentUrl:       parentUrl,
	}

	return independent, nil
}

// AddController of category
func (independent *Service) AddController(category string, controller handler.Interface) {
	independent.Controllers.Set(category, controller)
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

func (independent *Service) IsProxyRequired(proxyUrl string) bool {
	for _, url := range independent.RequiredProxies {
		if strings.Compare(url, proxyUrl) == 0 {
			return true
		}
	}

	return false
}

// A Pipeline creates a chain of the proxies.
func (independent *Service) Pipeline(pipeEnd *pipeline.PipeEnd, proxyUrls ...string) error {
	pipelines := independent.pipelines
	controllers := independent.Controllers
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
	for _, controllerInterface := range independent.Controllers {
		c := controllerInterface.(handler.Interface)
		extensions = append(extensions, c.RequiredExtensions()...)
	}

	return extensions
}

func (independent *Service) createHandlerConfiguration() error {
	// validate the Controllers
	for category, controllerInterface := range independent.Controllers {
		c := controllerInterface.(handler.Interface)

		controllerConfig := handlerConfig.NewController(c.ControllerType(), category)

		sourceInstance, err := handlerConfig.NewInstance(controllerConfig.Category)
		if err != nil {
			return fmt.Errorf("service.NewInstance: %w", err)
		}
		controllerConfig.Instances = append(controllerConfig.Instances, *sourceInstance)
		independent.Config.SetController(controllerConfig)
	}
	return nil
}

func (independent *Service) createConfiguration() error {
	independent.Config = config.Empty(independent.id, independent.url, config.IndependentType)

	if err := independent.createHandlerConfiguration(); err != nil {
		return fmt.Errorf("createHandlerConfiguration: %w", err)
	}

	return nil
}

// prepareConfiguration prepares the configuration.
func (independent *Service) prepareConfiguration(expectedType config.Type) error {
	// validate the service itself
	if err := independent.createConfiguration(); err != nil {
		return fmt.Errorf("independent.createConfiguration: %w", err)
	}

	if err := independent.fillConfiguration(); err != nil {
		return fmt.Errorf("independent.fillConfiguration: %w", err)
	}

	return nil
}

func (independent *Service) fillConfiguration() error {
	exist, err := config.FileExist()
	if err != nil {
		return fmt.Errorf("config.Exist: %w", err)
	}
	if !exist {
		return nil
	}
	config.SetDefault(independent.ctx.Config())
	config.RegisterPath(independent.ctx.Config())

	serviceConfig, err := config.Read(independent.ctx.Config())
	if err != nil {
		return fmt.Errorf("config.Read: %w", err)
	}

	if serviceConfig.Id != independent.Config.Id {
		return fmt.Errorf("service type is overwritten. expected '%s', not '%s'", independent.Config.Type, serviceConfig.Type)
	}

	// validate the Controllers
	for category, controllerInterface := range independent.Controllers {
		c := controllerInterface.(handler.Interface)

		controllerConfig, err := serviceConfig.GetController(category)
		if err != nil {
			return fmt.Errorf("serviceConfig.GetController(%s): %w", category, err)
		}

		if controllerConfig.Type != c.ControllerType() {
			return fmt.Errorf("handler expected to be of '%s' type, not '%s'", c.ControllerType(), controllerConfig.Type)
		}

		if len(controllerConfig.Instances) == 0 {
			return fmt.Errorf("missing %s handler instances", category)
		}

		if controllerConfig.Instances[0].Port == 0 {
			return fmt.Errorf("the port should not be 0 in the source")
		}
	}

	independent.Config = serviceConfig

	return nil
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

// onClose closing all the dependencies in the orchestra.
func (independent *Service) onClose(request message.Request, logger *log.Logger, _ ...*client.ClientSocket) message.Reply {
	logger.Info("service received a signal to close",
		"service", independent.Config.Url,
		"todo", "close all controllers",
	)

	for name, controllerInterface := range independent.Controllers {
		c := controllerInterface.(handler.Interface)
		if c == nil {
			continue
		}

		// I expect that the killing process will release its resources as well.
		err := c.Close()
		if err != nil {
			logger.Error("handler.Close", "error", err, "handler", name)
			request.Fail(fmt.Sprintf(`handler.Close("%s"): %v`, name, err))
		}
		logger.Info("handler was closed", "name", name)
	}

	// remove the orchestra lint
	independent.Context = nil

	logger.Info("all controllers in the service were closed")
	return request.Ok(key_value.Empty())
}

// runManager the orchestra in the background. If it failed to run, then return an error.
// The url request is the main service to which this orchestra belongs too.
//
// The logger is the handler logger as it is. The orchestra will create its own logger from it.
func (independent *Service) runManager() error {
	replier, err := handler.SyncReplier(independent.Logger.Child("manager"))
	if err != nil {
		return fmt.Errorf("handler.SyncReplierType: %w", err)
	}

	conf := config.InternalConfiguration(config.ManagerName(independent.Config.Url))
	replier.AddConfig(conf, independent.Config.Url)

	closeRoute := command.NewRoute("close", independent.onClose)
	err = replier.AddRoute(closeRoute)
	if err != nil {
		return fmt.Errorf(`replier.AddRoute("close"): %w`, err)
	}

	independent.manager = replier
	go func() {
		if err := independent.manager.Run(); err != nil {
			independent.Logger.Fatal("service.manager.Run: %w", err)
		}
	}()

	return nil
}

// Prepare the services by validating, linting the configurations, as well as setting up the dependencies
func (independent *Service) Prepare() error {
	if len(independent.Controllers) == 0 {
		return fmt.Errorf("no Controllers. call service.AddController")
	}

	// validate the service itself
	if err := independent.createConfiguration(); err != nil {
		return fmt.Errorf("independent.createConfiguration: %w", err)
	}

	if err := independent.fillConfiguration(); err != nil {
		return fmt.Errorf("independent.fillConfiguration: %w", err)
	}

	return nil
}

// RunManager the services by validating, linting the configurations, as well as setting up the dependencies
func (independent *Service) RunManager(as config.Type) error {
	if len(independent.Controllers) == 0 {
		return fmt.Errorf("no Controllers. call service.AddController")
	}

	requiredExtensions := independent.requiredControllerExtensions()

	//
	// prepare the config with the service, it's controllers and instances.
	// it doesn't prepare the proxies, pipelines and extensions
	//----------------------------------------------------
	err := independent.prepareConfiguration(as)
	if err != nil {
		return fmt.Errorf("prepareConfiguration: %w", err)
	}

	//
	// prepare the orchestra for dependencies
	//---------------------------------------------------
	err = independent.Context.Run(independent.Logger)
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
	for name, controllerInterface := range independent.Controllers {
		c := controllerInterface.(handler.Interface)
		var controllerConfig *handlerConfig.Handler
		var controllerExtensions []string

		controllerConfig, err = independent.Config.GetController(name)
		if err != nil {
			err = fmt.Errorf("c '%s' registered in the service, no config found: %w", name, err)
			goto closeContext
		}

		c.AddConfig(controllerConfig, independent.Config.Url)
		controllerExtensions = c.RequiredExtensions()
		for _, extensionUrl := range controllerExtensions {
			requiredExtension := independent.Config.GetExtension(extensionUrl)
			req := &clientConfig.Client{
				Url:  requiredExtension.Url,
				Id:   requiredExtension.Id,
				Port: requiredExtension.Port,
			}
			c.AddExtensionConfig(req)
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

// BuildConfiguration is invoked from Run. It's passed if the --build-config flag was given.
// This function creates a yaml config with the service parameters.
func (independent *Service) BuildConfiguration() {
	if !arg.Exist(arg.BuildConfiguration) {
		return
	}
	relativePath, err := arg.Value(arg.Path)
	if err != nil {
		independent.Logger.Fatal("requires 'path' flag", "error", err)
	}

	url, err := arg.Value(arg.Url)
	if err != nil {
		independent.Logger.Fatal("requires 'url' flag", "error", err)
	}

	execPath, err := path.GetExecPath()
	if err != nil {
		independent.Logger.Fatal("path.GetExecPath", "error", err)
	}

	outputPath := path.GetPath(execPath, relativePath)

	independent.Config.Url = url

	err = independent.Context.SetConfig(outputPath, independent.Config)
	if err != nil {
		independent.Logger.Fatal("failed to write the proxy into the file", "error", err)
	}

	independent.Logger.Info("yaml config was generated", "output path", outputPath)

	os.Exit(0)
}

// Run the service.
func (independent *Service) Run() {
	independent.BuildConfiguration()
	var wg sync.WaitGroup

	err := independent.runManager()
	if err != nil {
		err = fmt.Errorf("service.runManager: %w", err)
		goto errOccurred
	}

	for name, controllerInterface := range independent.Controllers {
		c := controllerInterface.(handler.Interface)
		if err = independent.Controllers.Exist(name); err != nil {
			independent.Logger.Error("service.Controllers.Exist", "config", name, "error", err)
			break
		}

		wg.Add(1)
		go func() {
			err = c.Run()
			wg.Done()

		}()
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

		independent.Logger.Fatal("one or more controllers removed, exiting from service", "error", err)
	}
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
			source.Instances[0].Port = proxyConfiguration.Instances[0].Port

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
			main.Instances[0].Port = extensionConfiguration.Port

			depConfig.SetController(main)

			err = dep.SetServiceConfig(depConfig)
			if err != nil {
				return fmt.Errorf("failed to update port in dependency extension: '%s': %w", dep.Url(), err)
			}
		}
	}

	return nil
}
