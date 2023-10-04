# Service
*This is one of the core modules.*

The *service* library allows creating of independent micro**services**.

> We omit the **micro** prefix from now on.

**The independent services are minimal stand-alone applications.**
A developer defines the list of API routes and a function that's executed.

The API routes are grouped into the [handlers](https://github.com/ahmetson/handler-lib).
*The handler defines how the API is served to the external users.*

The independent services are isolated from each other.
They don't share a configuration nor the data that they are working on.

## Application architecture

The application consists of multiple services.

The independent service is the core of the application.
As it will keep the business logic.

There are auxiliary services.

The proxies that operate on a request before it's passed to a service.
And extensions extending service possibility or do some side works.

**One of the aims of SDS framework is to write self-orchestrating applications.**

In the SDS framework, the services are organized in a parent-child relationship.
The independent service acts as a root node. 
The auxiliary services counted as the child nodes.

That means the root node is responsible for spawning the children.

It's done in the background by the framework, so developers don't have to worry about it.

## Coarsely grained API

When you define an API routes, you set them at the minimal level.
But if the service is stored on another machine, then requesting data by minimal API will cause a delay.

In the microservice architecture, the remote APIs must be coarsely grained to reduce the network hops.

The extensions with *merge* flag can group the routes of the parent.

# LifeCycle
When a service runs, it prepares itself.
The preparation is composed of two steps.
First, it creates a configuration based on the required parameters.
Then, it fills them. 
If a user passed a pre-defined configuration,
then the process will validate and apply them.
If there is no configuration, then it will create a random configuration.
That random configuration is written to the path.

When the service is prepared, it runs the manager. 
The manager is set in the *"PREPARED"* state. 
If the service has a parent service, then it will send the message to the parent.

After running the manager, the service runs the dependency manager.
The dependency manager running means three things.
If the service is running, it will ask to acknowledge the parent.
Acknowledging the means to ask permission to connect from the parent.
If the service is not acknowledged, it will mark that service as failed.
If the service is not running, it will check the binary.
If the binary exists, the service will run that binary.
If the binary does not exist, the dependency manager will install it.
Then run it.
The dependency manager is working with the nearby proxies and extensions.

> As a parent, it passes an id, configuration and its own id as a parent.

When the dependencies are all set, it updates the state to *"READY"*.
When some dependencies are not set, it will mark itself as *"PARTIALLY_READY"*.

When it's (partially) ready, the dependency manager creates a heartbeat tracker.
This tracker is given to the manager.
Then, the manager creates its own heartbeat, and sends the messages to the parent.
If no parent is given, it won't set it.

# Usage

```go
id := "application name"
s := service.New(id)
s.Prepare() // runs the manager
s.RunDepManager()
s.Run() // sets up
```

# Service Lib

The proxies could be used for validation, authentication, authorization, load-balancing.

Proxies can be organized as a chain of proxies.
The proxy B receives a message only if proxy A succeeded.

As auxiliary service, proxies must always forward the messages.
Therefore, proxies must have the destination to forward the message.

## Parent-child relationship
Even though the proxies are spawned by the parent proxies.
There is possible to create a proxy that will manage the parent too.
For example, the proxies of the *rely* category could manage the independent services.

## Category
When there is a proxy chain, there must be some order of the proxies.

By behavior, the proxies are classified into categories.
And the proxies are ordered by default on its category.

The categories are:
* *entry* &ndash; proxies set a different protocol for receiving data. Use `PairExternal` method.
* *authn* &ndash; proxies authorize the requester
* *authr* &ndash; proxies set the role-based access
* *valid* &ndash; proxies validate the message
* *convert* &ndash; proxies serialize/deserialize the messages. Use `SetMessageOperations` method.
* *reliability* &ndash; proxies are responsible for service availability. Examples are backup, load balancing.

The default order in which proxies are chained is

> **source** >
> entry > convert >
> authn > valid >
> authr > rely >
> **destination**

## Definition
There are two ways to define proxies.
At the independent service also called *built in definition*.
Or, using the `meta` interface also called *on the fly*.

If the destination service has the proxies, then proxies are added in order by [category](#category).

> **Todo**
>
> `meta` interface is not yet developed.
> The idea of the meta is to access to the service including its context.

Refer to [build in definition](BUILT_IN_DEFINE.md) for description on how to add proxies.


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
