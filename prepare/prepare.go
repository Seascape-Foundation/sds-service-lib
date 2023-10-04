// Package prepare creates the configuration, then fills it.
package prepare

import (
	"fmt"
	"github.com/ahmetson/service-lib"
)

// Prepare the services by validating, linting the configurations, as well as setting up the dependencies
func Prepare(independent *service.Service) error {
	if len(independent.Handlers) == 0 {
		return fmt.Errorf("no Handlers. call service.SetHandler")
	}

	// validate the service itself
	if err := createConfiguration(independent); err != nil {
		return fmt.Errorf("independent.createConfiguration: %w", err)
	}

	if err := fillConfiguration(independent); err != nil {
		return fmt.Errorf("independent.fillConfiguration: %w", err)
	}

	//exist, err := flag.FileExist()
	//if err != nil {
	//	return fmt.Errorf("flag.FileExist: %w", err)
	//}
	//if !exist {
	//	if err := writeConfiguration(independent); err != nil {
	//		return fmt.Errorf("writeConfiguration: %w", err)
	//	}
	//}

	return nil
}

func createConfiguration(independent *service.Service) error {
	//independent.flag = flag.Empty(independent.Id(), independent.Url(), flag.IndependentType)
	//
	//if err := createHandlerConfiguration(independent); err != nil {
	//	return fmt.Errorf("createHandlerConfiguration: %w", err)
	//}
	//
	return nil
}

func createHandlerConfiguration(independent *service.Service) error {
	// validate the Handlers
	//for category, controllerInterface := range independent.Handlers {
	//c := controllerInterface.(handler.Interface)
	//
	//controllerConfig := handlerConfig.NewController(c.ControllerType(), category)
	//
	//sourceInstance, err := handlerConfig.NewInstance(controllerConfig.Category)
	//if err != nil {
	//	return fmt.Errorf("service.NewInstance: %w", err)
	//}
	//controllerConfig.Instances = append(controllerConfig.Instances, *sourceInstance)
	//independent.flag.SetController(controllerConfig)
	//}
	return nil
}

func fillConfiguration(independent *service.Service) error {
	//exist, err := flag.FileExist()
	//if err != nil {
	//	return fmt.Errorf("flag.Exist: %w", err)
	//}
	//if !exist {
	//	return nil
	//}
	//
	//flag.SetDefault(independent.ConfigEngine())
	//flag.RegisterPath(independent.ConfigEngine())
	//
	//serviceConfig, err := flag.Read(independent.ConfigEngine())
	//if err != nil {
	//	return fmt.Errorf("flag.Read: %w", err)
	//}
	//
	//if serviceConfig.Id != independent.flag.Id {
	//	return fmt.Errorf("service type is overwritten. expected '%s', not '%s'", independent.flag.Type, serviceConfig.Type)
	//}
	//
	// validate the Handlers
	//for category, controllerInterface := range independent.Handlers {
	//	c := controllerInterface.(handler.Interface)
	//
	//	controllerConfig, err := serviceConfig.GetController(category)
	//	if err != nil {
	//		return fmt.Errorf("serviceConfig.GetController(%s): %w", category, err)
	//	}
	//
	//	if controllerConfig.Type != c.ControllerType() {
	//		return fmt.Errorf("handler expected to be of '%s' type, not '%s'", c.ControllerType(), controllerConfig.Type)
	//	}
	//
	//	if len(controllerConfig.Instances) == 0 {
	//		return fmt.Errorf("missing %s handler instances", category)
	//	}
	//
	//	if controllerConfig.Instances[0].Port == 0 {
	//		return fmt.Errorf("the port should not be 0 in the source")
	//	}
	//}

	//independent.flag = serviceConfig

	return nil
}

// writeConfiguration is saving the configuration.
// This function creates a yaml flag with the service parameters.
func writeConfiguration(independent *service.Service) error {
	//outputPath := flag.GetPath(independent.ConfigEngine())
	//
	//err := independent.Context.SetConfig(outputPath, independent.flag)
	//if err != nil {
	//	return fmt.Errorf("failed to write the proxy into the file: %w", err)
	//}

	return nil
}
