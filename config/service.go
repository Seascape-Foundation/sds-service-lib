package config

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/config-lib"
	handlerConfig "github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/os-lib/arg"
	"github.com/ahmetson/os-lib/path"
	"github.com/ahmetson/service-lib/config/service"
	"github.com/ahmetson/service-lib/config/service/pipeline"
)

const (
	IdFlag     = "id"
	UrlFlag    = "url"
	ParentFlag = "parent"
	ConfigFlag = "config"
)

// Service type defined in the config
type Service struct {
	Type        Type
	Url         string
	Id          string
	Controllers []*handlerConfig.Handler
	Proxies     []*service.Proxy
	Extensions  []*service.Extension
	Pipelines   []*pipeline.Pipeline
}

type Services []Service

func Empty(id string, url string, serviceType Type) *Service {
	return &Service{
		Type:        serviceType,
		Id:          id,
		Url:         url,
		Controllers: make([]*handlerConfig.Handler, 0),
		Proxies:     make([]*service.Proxy, 0),
		Extensions:  make([]*service.Extension, 0),
		Pipelines:   make([]*pipeline.Pipeline, 0),
	}
}

// FileExist checks is there any configuration given
func FileExist() (bool, error) {
	execPath, err := path.GetExecPath()
	if err != nil {
		return false, fmt.Errorf("path.GetExecPath: %w", err)
	}

	configPath := ""
	if arg.Exist(ConfigFlag) {
		configPath, err = arg.Value(ConfigFlag)
		if err != nil {
			return false, fmt.Errorf("failed to get the config path: %w", err)
		}
	} else {
		configPath = "service.yml"
	}

	absPath := path.GetPath(execPath, configPath)
	exists, err := path.FileExists(absPath)
	if err != nil {
		return false, fmt.Errorf("path.FileExists('%s'): %w", absPath, err)
	}

	return exists, nil
}

func SetDefault(engine config.Interface) {
	execPath, _ := path.GetExecPath()
	engine.SetDefault("SERVICE_CONFIG_NAME", "service")
	engine.SetDefault("SERVICE_CONFIG_PATH", execPath)
}

// RegisterPath sets the path to the yaml file
func RegisterPath(engine config.Interface) {
	if !arg.Exist(ConfigFlag) {
		return
	}
	execPath, _ := path.GetExecPath()

	configurationPath, _ := arg.Value(ConfigFlag)

	absPath := path.GetPath(execPath, configurationPath)

	dir, fileName := path.SplitServicePath(absPath)
	engine.Set("SERVICE_CONFIG_NAME", fileName)
	engine.Set("SERVICE_CONFIG_PATH", dir)
}

func Read(engine config.Interface) (*Service, error) {
	configName := engine.GetString("SERVICE_CONFIG_NAME")
	configPath := engine.GetString("SERVICE_CONFIG_PATH")
	configExt := "yaml"

	value := key_value.Empty().Set("name", configName).
		Set("type", configExt).
		Set("configPath", configPath)

	file, err := engine.Read(value)
	if err != nil {
		return nil, fmt.Errorf("engine.ReadValue(%s/%s.%s): %w", configPath, configName, configExt, err)
	}

	serv, ok := file.(*Service)
	if !ok {
		return nil, fmt.Errorf("'%s/%s.%s' not a valid Service", configPath, configName, configExt)
	}

	return serv, nil
}

func (s *Service) PrepareService() error {
	err := s.ValidateTypes()
	if err != nil {
		return fmt.Errorf("service.ValidateTypes: %w", err)
	}
	err = s.Lint()
	if err != nil {
		return fmt.Errorf("service.Lint: %w", err)
	}

	return nil
}

// UnmarshalService decodes the yaml into the config.
func UnmarshalService(services []interface{}) (*Service, error) {
	if len(services) == 0 {
		return nil, nil
	}

	kv, err := key_value.NewFromInterface(services[0])
	if err != nil {
		return nil, fmt.Errorf("failed to convert raw config service into map: %w", err)
	}

	var serviceConfig Service
	err = kv.Interface(&serviceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to convert raw config service to config.Service: %w", err)
	}
	err = serviceConfig.PrepareService()
	if err != nil {
		return nil, fmt.Errorf("prepareService: %w", err)
	}

	return &serviceConfig, nil
}

// Lint sets the reference to the parent from the child.
//
// If the child config is used independently, then
// there is no way to know to which parent it belongs too.
//
// In this case, it sets the reference to the handler from the handler reference.
// If the handler instances are used independently, then other services may know to which service they belong too.
func (s *Service) Lint() error {
	// Lint handler instances to the controllers
	for cI, c := range s.Controllers {
		for iI, instance := range c.Instances {
			if len(instance.ControllerCategory) > 0 {
				if instance.ControllerCategory != c.Category {
					return fmt.Errorf("invalid name for handler instance. "+
						"In service instance '%s', handler '%s', instance '%s'. "+
						"the '%s' name in the handler instance should be '%s'",
						s.Id, c.Category, instance.Id, instance.ControllerCategory, c.Category)
				} else {
					continue
				}
			}

			instance.ControllerCategory = c.Category
			c.Instances[iI] = instance
		}

		s.Controllers[cI] = c
	}

	return nil
}

// ValidateTypes the parameters of the service
func (s *Service) ValidateTypes() error {
	if err := ValidateServiceType(s.Type); err != nil {
		return fmt.Errorf("identity.ValidateServiceType: %v", err)
	}

	for _, c := range s.Controllers {
		if err := handlerConfig.ValidateControllerType(c.Type); err != nil {
			return fmt.Errorf("handler.ValidateControllerType: %v", err)
		}
	}

	return nil
}

// GetController returns the handler config by the handler name.
// If the handler doesn't exist, then it returns an error.
func (s *Service) GetController(name string) (*handlerConfig.Handler, error) {
	for _, c := range s.Controllers {
		if c.Category == name {
			return c, nil
		}
	}

	return nil, fmt.Errorf("'%s' handler was not found in '%s' service's config", name, s.Url)
}

// GetControllers returns the multiple controllers of the given name.
// If the controllers don't exist, then it returns an error
func (s *Service) GetControllers(name string) ([]*handlerConfig.Handler, error) {
	controllers := make([]*handlerConfig.Handler, 0, len(s.Controllers))
	count := 0

	for _, c := range s.Controllers {
		if c.Category == name {
			controllers[count] = c
			count++
		}
	}

	if len(controllers) == 0 {
		return nil, fmt.Errorf("no '%s' controlelr config", name)
	}
	return controllers, nil
}

// GetFirstController returns the handler without requiring its name.
// If the service doesn't have controllers, then it will return an error.
func (s *Service) GetFirstController() (*handlerConfig.Handler, error) {
	if len(s.Controllers) == 0 {
		return nil, fmt.Errorf("service '%s' doesn't have any controllers in yaml file", s.Url)
	}

	controller := s.Controllers[0]
	return controller, nil
}

// GetExtension returns the extension config by the url.
// If the extension doesn't exist, then it returns nil
func (s *Service) GetExtension(url string) *service.Extension {
	for _, e := range s.Extensions {
		if e.Url == url {
			return e
		}
	}

	return nil
}

// GetProxy returns the proxy by its url. If it doesn't exist, returns nil
func (s *Service) GetProxy(url string) *service.Proxy {
	for _, p := range s.Proxies {
		if p.Url == url {
			return p
		}
	}

	return nil
}

// SetProxy will set a new proxy. If it exists, it will overwrite it
func (s *Service) SetProxy(proxy *service.Proxy) {
	existing := s.GetProxy(proxy.Url)
	if existing == nil {
		s.Proxies = append(s.Proxies, proxy)
	} else {
		*existing = *proxy
	}
}

// SetExtension will set a new extension. If it exists, it will overwrite it
func (s *Service) SetExtension(extension *service.Extension) {
	existing := s.GetExtension(extension.Url)
	if existing == nil {
		s.Extensions = append(s.Extensions, extension)
	} else {
		*existing = *extension
	}
}

// SetController adds a new handler. If the handler by the same name exists, it will add a new copy.
func (s *Service) SetController(controller *handlerConfig.Handler) {
	s.Controllers = append(s.Controllers, controller)
}

func (s *Service) SetPipeline(pipeline *pipeline.Pipeline) {
	s.Pipelines = append(s.Pipelines, pipeline)
}

func (s *Service) HasProxy() bool {
	return len(s.Proxies) > 0
}
