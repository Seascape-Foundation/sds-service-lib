# Proxy
Proxy is a special type of service that sits at the front of independent services.
The proxies do a pre-computation such as validation, authentication, authorization, load-balancing and many more.

Proxies can be organized as a chain of proxies.
However, the destination of the proxy must be pointing always to the independent service routes.


## Category
By behavior, the proxies are classified into categories:
* *entry* &ndash; proxies set a different protocol for receiving data
* *authn* &ndash; proxies authorize the requester
* *authr* &ndash; proxies set the role-based access
* *valid* &ndash; proxies validate the message
* *convert* &ndash; proxies serialize/deserialize the messages.
* *reliability* &ndash; proxies are responsible for service availability. Examples are backup, load balancing.

The default order in which proxies are chained is

> **source** >
> entry > convert >
> authn > valid >
> authr > rely >
> **destination**

## Definition
One of the aims of SDS framework is to write self-managing applications.

The independent service as the keeper of the business logic is the core of the application.
On the other hand, proxy-chain could be updated on the fly.

This is the reason why proxies are defined in the independent service.
When starts, the independent service will run the proxies as well.


### Url
We start defining the proxy by its url, which happens in the independent service.

```go
webProxy := config.NewProxy("url")
sshProxy := config.NewProxy("url")
```

### Destination
Then, we have to define the destination.

Proxies can apply themselves to the whole service, to the specific handler or to the specific route.
The route is the minimal destination.

> It's not a final type of destination.
> The proxy will ask the destination for its id and duplicates.
> If the duplicates are given, then it will send by round-robin format.
>
> This will allow, for example, two identical handlers, or two identical services to be supported by the proxy.

The following line of code defines a destination with one route.

```go
package readme
// config package is defined in service-lib/config
destinations := config.NewDestination(
	[]string{"url"}, 
	[]string{"category_1", "category_2"},
	[]string{"route_1"})
```

The `url` parameter is optional. Without it, the current independent service is counted as the destination.

The unit parameters are joint by Union set. 
The code above will search for `url.category_1.route_1` and `url.category_2.route_1`.

Listing all routes explicitly is tiresome. The other way to define routes is by handler name.

```go
destinations = config.NewHandlerDestination([]string{"url"}, []string{"category"})
```

The url argument is optional. If it wasn't given, the current service is counted as the destination.

When you are listing the handler as the destination, sometimes you want to exclude some routes.
It's done by `Exclude`:

```go
destinations.Exclude("command_1", "command_2", ...)
```

Or excluded by other destinations.

```go
destinations.Exclude(sub_destinations)
```

### Recap of destination

* Independent service manages the proxy.
* Proxies for the service are defined in the service.
* Definition always must point the service as the destination.
* Proxy could be applied to the all services, to the handlers or to some routes.
* Proxy can organize a chain.
* Destinations can be defined as the routes or as the handlers.
* The following route destinations are identical:
* - `config.NewDestination(["url_1"], ["category_1"], ["cmd_1"])`
* The following handler destinations are identical:
* - `config.NewHandlerDestination(["url_1"], ["category_1"])`
* In both functions, the url parameter is optional
* Some destinations might be excluded:
* - `destinations.Exclude("command_1")`
* - `destinations.ExcludeByUnit(subDestinations)`


### Source
The proxy receives the data from anyone who has access to the machine.
To narrow the access, we can set the sources.

The sources define the list of the url or at least a one. 
Then only the services of the url could access to the proxy.

### Register
Finally, the prepared proxy, destination and optional source is set in the independent service as a 
proxy chain:

```go
proxyChain := service.NewProxyChain(source1, [proxy1, proxy2], destination)

service := independent.New("id")
service.SetProxyChains(proxyChain)
```

If request to `proxy1` is passing, then a message is forwarded to `proxy2`.

### Recap of registering

The example code of proxy registering:

```go
// defining dependency
webProxy        := config.NewProxy("url") 
sshProxy        := config.NewProxy("url") 
scheduleProxy   := config.NewProxy("url")
plainProxy      := config.NewProxy("url")

// defining destinations
manageRoutes    := config.NewDestination("main", ["admin_set_1", "admin_set_2"])
announceRoute   := config.NewDestination("main", "announce")
allRoutes       := config.NewHandlerDestination("main")
publicRoutes    := allRoutes.Exclude(announceRoute, manageRoutes)

userChain := config.NewProxyChain([webProxy, plainProxy], publicRoutes)
managerChain := config.NewProxyChain([sshProxy, plainProxy], manageProxy)
announceChain := config.NewProxyChain(service.Url(), scheduleProxy, announceRoute)

// todo linter adds an explanation:
err := service.SetProxyChain(userChain, managerChain, announceChain)
```

Setting the explicit sources will allow access to the first proxy from the list of sources only.

This is the end of proxy definitions.
The next section will explain how to define the proxy itself.
All the definitions are defined by the `config-lib` and `service-lib`.
The actual implementation of the proxy is defined in the `proxy-lib` module.

> Checking the source is done by the security layer.

> **todo**
> 
> implement auto-chain by proxy id

## Execution
The independent service will run the last proxy.
The parent will pass to the proxy the id, url, parent id, and link to the configuration.
The proxy will generate its configuration.
The last proxy will start the next service in the chain.
The steps are repeated until it will reach the first proxy.

When the proxy chain was defined, the destination was declared as a rule.
However, it doesn't contain the information about the handler endpoint,
or the list of the matching and excluded routes.

The proxy will get the destination and source information.
The proxy will get the handler information from the parent.
It will then get the information about the routes, excluded routes as well from parent.

> The excluded routes are returned if the routes are empty.

## Multi source
If the service has a multiple allowed service, then they are assisted.

> It's enabled when the security socket is enabled

## Multi destination
If the multiple destinations are given, then they are passed as the array to the handle function.
Users are able to choose any destination by its index to return back to the user.

Return an empty index list to let it choose for the proxy.
Or return the selected indexes.

The multiple destinations are of the same level.

## Implementation

### Definition
The declaration of the proxy parameters is defined in the `config-lib/service` package.
Because, the proxies are stored in the configuration as a yaml file. 
They maybe used later.

The ``

The `service.NewProxy(url string)` returns the pointer to `service.Proxy`.

The proxy structure is:

```go
type Proxy struct{
	Url string
	Id string
	Category string `json:"_,omitempty"`
}
```

Since the proxy sets the Url, the id is generated from the url itself.

The destination structure defined as `service.Rule` in the `config-lib` module:

```go
package readme
type Rule struct{
	Urls                []string // the service url 
	Categories          []string // handler category 
	Commands            []string    // route name 
	ExcludedCommands    []string
}
```

The sources are defined as a list of strings.

The proxy chains are defined as `service.ProxyChain` in the `config-lib` module.

```go
type ProxyChain struct {
	Sources []string
	Proxies []*Proxy
	Destinations *Rule
}
```

When a proxy is set to the service, the service will have store them as

```go
type Service struct{
	ProxyChains []*ProxyChain
}
```

> The category is not implemented yet.

> **todo**
> 
> Generate the id from proxy-chain, destination.

---

## Execution
The independent service manager has the following routes:
* `handlers` returns the list of the handler configuration.
* `routes` returns the list of the routes in the configuration.
* `proxy-chain` returns the proxy chain


---
The above code described the proxy definition in the `config-lib` and `service-lib`.
Now, it's the time to define the proxy within the `proxy-lib` itself.

# Proxy

Because, proxies are the pre-layer of the independent services.
For the end users, the proxies must be invisible.
Therefore, the proxies will define the handlers based on the destinations.
As a developer, you can not set them.

*For example, if the destination handler is trigger-able, then the proxy will be trigger-able as well.*
*Or, if the destination handler does not reply back, then proxy won't reply back as well.*

The proxy purposes are to extend the services, therefore, some parts of the handlers are over-writable.

## Over-writable

A developer can over-write three parts of the handlers.
The first part is the handler's frontend.
This allows supporting any kind of protocol; that's different from zeromq.

> SDS uses zeromq for internal communication.
> 
> But the application may want to enable the HTTP, WebSocket or other kinds of protocols for the users.

The over-writing the frontend is done via `proxy.PairExternal(pair.Interface)`

---

The second part is the message format.
This allows supporting any kind of messages; that's different from `message.DefaultMessage()`

> SDS uses the messages defined as `message.Request` and `message.Reply`.
> These messages are sent in the serialized JSON format.
>
> But the application may accept a plain text, or BSON, or a custom format.

It's done via `proxy.SetMessageOperations(*message.Operations)`
The over-written message types are accepted only by the proxy itself.
When a proxy forwards the message, it will convert it to the SDS default message format.

If the destination handler must reply a message, then proxy must reply as well.
In this scenario,
the message that comes from the destination is converted into the custom type using `messageOp.NewReply`

Some proxies maybe working with the metadata. 
As such, they won't need to process the messages.
For this kind of proxies, set the message type as `message.RawMessage()`.

> **Todo**
> 
> Maybe add another message operation to convert the message to destination's format?

## Handle Function
As a pre-layer of the independent services, the proxies are either forwarding messages or rejects them.
Such that, the proxies don't support the handler's routing behavior.
Instead, the proxies define another handle function.
The handle function is `func(req message.RequestInterface) (message.RequestInterface, error)`.

The `req` is the message that was requested by the user.
If message operations are over-written, then a request message will be created with that.

Depending on the execution result, proxy forwards the message to the next unit.
Or it rejects it.
To indicate the target, the rejection reason, a developer returns an error as well.

The handle function must be set.
It's set by calling `proxy.SetRequestHandleFunc()`

The other handle function is `func(rep message.ReplyInterface) (message.ReplyInterface, error)`.
It's set by calling `proxy.SetReplyHandleFunc()`.

## Sender
Besides, the over-writing the handle functions, it's possible to over-write the Sender.

`proxy.SetDestinationSender(func(dest []*Dest, req message.RequestInterface)) ([]*Dest, error)`
`proxy.SetSourceSender(func(clientId string, sourceIds []string, dest []*Dest, reply message.ReplyInterface)) ([]string, error)`

Over-writing the senders could be used, for example, to announce all users.
Or for load-balancing.

> **todo**
> 
> Over-write the senders to include the service availability information.
> Or maybe design it to work with the extension that has the metadata of the services.
> 
> It will be designed by implementing backup-proxy, load-balancer-proxy.

## Recap

```go
package readme

func main() {
	// Since it's called from the parent, the parent will pass the flags
	proxy := proxy.New()
	proxy.PairExternal() // optional
	proxy.SetMessageOperations() // optional
	proxy.SetHandleFunc()
	
	// runChan blocks the service until proxy won't stop.
	// In case if the proxy stopped with an error, the channel shall contain it.
	// In case if the proxy closed by the destination, then it will return nil.
	runChan, startErr := proxy.Start()
	if startErr != nil {
		panic(startErr)
    }
	
	err := <- runChan
	if err != nil {
		panic(err)
    }
}
```

---

# Proxy app

Let's create `main.go` with the minimal *proxy*:

```go
package main

import (
"github.com/ahmetson/service-lib"
log "github.com/ahmetson/log-lib"
"github.com/ahmetson/service-lib/configuration"
"github.com/ahmetson/service-lib/proxy"
)

func main() {
logger, _ := log.New("my-proxy", false)
appConfig, _ := configuration.New(logger)

// setup service
service := proxy.New(appConfig, logger.Child("proxy"))

// setup a default source
service.SetDefaultSource(configuration.ReplierType)
// or
// service.SetCustomSource(customController)

// destinations, handlers are part of the handler
service.Controller.RequireDestination(configuration.ReplierType)
service.Controller.SetRequestHandler()
service.Controller.SetReplyHandler()

// validate before running it
service.Prepare()

service.Run()
}
```

> TODO
>
> Before adding source, add the extensions
> That means, we need to change the request handler, reply handler.
> The handlers in the controller should receive extensions.
> ```go
> // List the dependency
> //service.RequireExtension("")
> ```

All services start with the configuration setup.
On `main.go` the configuration is set to `appConfig`.

Then we do:
* Initialize a new proxy
* Set up: source, destination and handlers.
* Prepare it by checking configurations.
* Finally, we start the service: `service.Run()`

## Source
The *proxy controller* will add itself to *source* controller automatically.
The handlers in the *proxy controller* should send the messages to *proxy controller*.

### Custom Source
The custom source controllers should implement the interface:
`github.com/ahmetson/service-lib/controller.Interface`.

> Check the example on https://github.com/ahmetson/web-proxy

## Request Handler
The request handler is the function of the type:

`github.com/ahmetson/service-lib/proxy.RequestHandler`

## Reply Handler

> This is optional

The reply handler is the function of the type:

`github.com/ahmetson/service-lib/proxy.ReplyHandler`
