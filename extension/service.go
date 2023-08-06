/*Package extension is used to scaffold the extension service
 */
package extension

import (
	"fmt"
	"github.com/ahmetson/service-lib/config"
	service2 "github.com/ahmetson/service-lib/config/service"
	"github.com/ahmetson/service-lib/log"
	"github.com/ahmetson/service-lib/server"
	"github.com/ahmetson/service-lib/service"
)

const defaultControllerName = "main"

type _service = service.Service

// Extension of the extension type
type Extension struct {
	*_service
}

// New extension service based on the configurations
func New(config *config.Config, parent *log.Logger) (*Extension, error) {
	logger := parent.Child("extension")

	base, err := service.New(config, logger)
	if err != nil {
		return nil, fmt.Errorf("service.New: %w", err)
	}

	service := Extension{
		_service: base,
	}

	return &service, nil
}

// AddController creates a server of this extension
func (extension *Extension) AddController(controllerType service2.ControllerType) error {
	if controllerType == service2.UnknownType {
		return fmt.Errorf("unknown server type can't be in the extension")
	}

	if controllerType == service2.SyncReplierType {
		replier, err := server.SyncReplier(extension._service.Logger)
		if err != nil {
			return fmt.Errorf("server.NewReplier: %w", err)
		}
		extension._service.AddController(defaultControllerName, replier)
	} else if controllerType == service2.ReplierType {
		//router, err := server.NewRouter(controllerLogger)
		//if err != nil {
		//	return fmt.Errorf("server.NewRouter: %w", err)
		//}
		//extension.ControllerCategory = router
	} else if controllerType == service2.PusherType {
		puller, err := server.NewPull(extension._service.Logger)
		if err != nil {
			return fmt.Errorf("server.NewPuller: %w", err)
		}
		extension._service.AddController(defaultControllerName, puller)
	}

	return nil
}

func (extension *Extension) GetController() server.Interface {
	controllerInterface, _ := extension._service.Controllers[defaultControllerName]
	return controllerInterface.(server.Interface)
}

func (extension *Extension) GetControllerName() string {
	return defaultControllerName
}

// Prepare the service by validating the config.
// If the config doesn't exist, it will be created.
func (extension *Extension) Prepare() error {
	if err := extension._service.Prepare(service2.ExtensionType); err != nil {
		return fmt.Errorf("service.Run as '%s' failed: %w", service2.ExtensionType, err)
	}

	if len(extension._service.Controllers) != 1 {
		return fmt.Errorf("extensions support one server only")
	}

	return nil
}

// Run the service.
func (extension *Extension) Run() {
	extension._service.Run()
}
