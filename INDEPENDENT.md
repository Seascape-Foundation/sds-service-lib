# Independent service
> Definition of the independent service is defined on the README here:
[README](README.md)

Create a new go project:

```sh
mkdir my-service
cd my-service
go mod init github.com/account/my-service
```

Get the `service-lib` module and `common-lib` module:

```sh
go get github.com/ahmetson/service-lib
go get github.com/ahmetson/common-lib
```

## Internal process
The independent service is composed of the controllers.
The controllers need the routes.

The route is composed of the command name, handling function and 
the extensions that the function requires.

The list of all extensions in the functions are set at the beginning.

## Configure

> configuration is optional.

Create the `service.yml` configuration.

```yaml
# Should have at least one service
Services:
  - Type: Independent     # We are defining the service service
    Url: github.com/ahmetson/     # Custom name of the service to classify it.
    Instance: unique-id   # Unique id through this config
    Controllers:
      - Name: Name        # Source handler
        Type: Replier     # The type of the handler.
        Instances:
          - Instance: unique-handler
            Port: 8082
      - Name: pub
        Type: Publisher
        Instances:
          - Instance: unique-publisher
            Port: 8083
    Proxies:
      - Url: "" # optional proxies that it depends on
    Pipeline:
      - "proxy->handler" # name of the proxy to bind to the handler name
    Extensions:
      - Url: "database"  # optional extension that it depends on.
                          # the extensions are passed to the handlers.
```

Few notes on the configuration.
The `Services.Type` should be `Independent`.
The independent services are listed as extensions.

---

# Independent app

Let's create `main.go` with the sample independent service:

```go
package main

import (
	"github.com/ahmetson/service-lib"
	"github.com/ahmetson/service-lib/configuration"
	log "github.com/ahmetson/log-lib"
	"github.com/ahmetson/service-lib/independent"
	"github.com/ahmetson/service-lib/controller"
	"github.com/account/my-service/handler"
)

func main() {
	logger, _ := log.New("app-name", false)
	appConfig, _ := configuration.New(logger)

	service, _ := independent.New(appConfig, logger.Child("handler"))
	
	// The extensions are defined in the controllers
	replier := handler.NewReplier(logger)
	service.AddController("replier", replier)

	// Define the required proxies
	proxyUrl := "github.com/ahmetson/web-proxy"
	service.RequireProxy(proxyUrl)
	
	// Assign the handler to the proxy
	service.Pipe(proxyUrl, "replierInstance")
	
	service.Prepare()
	service.Run()
}
```

As all services, the service will require initiation of the configuration.
In the code above, the initiated config is assigned to `appConfig`.

Next we need to define our independent service.
Independent service will require controllers as part of its configuration.

We prepare the service by checking all controller parameters.
Finally, we run the service itself.

The controller is set defined on another code.

## Controller

For the example, we use the simplest type of the controller.
Let's create it on `handler/handler.go`:

```go
package handler

import (
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/service-lib/controller"
	"github.com/ahmetson/service-lib/communication/command"
	"github.com/ahmetson/service-lib/configuration"
	log "github.com/ahmetson/log-lib"
	"github.com/ahmetson/service-lib/remote"
)

var counter uint64 = 0

func onSetCounter(_ message.Request, _ *log.Logger, _ ...*remote.ClientSocket) message.Reply {
	newValue, _ := request.Parameters.GetUint64("counter")

	counter = newValue

	return message.Reply{
		Status:     message.OK,
		Message:    "",
		Parameters: key_value.Empty,
	}
}

func onGetCounter(message.Request, _ * log.Logger, _ ...*remote.ClientSocket) message.Reply{
    parameters := key_value.Empty()
	parameters.Set("counter", counter)
	return message.Reply{
		Status: message.OK,
		Message: "",
		Parameters: parameters,
    }
}

func NewReplier(logger*log.Logger) (*controller.Controller, error){
	replier, err := controller.NewReplier(logger)
	if err != nil {
		return nil, err
    }

	// extensions
	db := "github.com/ahmetson/mysql-extension"

	replier.RequireExtension(db)

	// routes
	setCounter := command.NewRoute("set_counter", onSetCounter, db)
	getCounter := command.NewRoute("get_counter", onGetCounter, db)

	replier.AddRoute(setCounter)
	replier.AddRoute(getCounter)
		
	return replier, nil
}
```

The handler defines the functions, and the controller.
We also list the required extensions by their urls.

