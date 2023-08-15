> Edit it readme later.

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

*Service Lib* is a library to create services on **SDS**. 

**SDS** is the protocol, libraries, and an account system.
The aim of it to create meaningful services and also useful for other developers.

The *account system* consists of the API service authentication and payment system.

## Contents

* [SDS](#service-lib)
* [Contents](#contents)
* [Components](#components)
* * [Service](#service)
* * * [Independent](#independent)
* * * [Extension](#extension)
* * * [Proxy](#proxy)
* * [Controller](#controller)
* * * [Replier](#replier)
* * * [Publisher](#publisher)
* * * [Puller](#puller)
* * * [Router](#router)
* * [Configuration](#configuration)
* * * [Env](#environment-variables)
* * * [Yaml](#yaml)
* [Flags](#service-flags)
* [Further Reading](#further-reading)

---

## Components

## Service
A **service** is a solution for a one problem as an independent
software. An **app** is an interconnection of the services. 

> Since services are independent software, then, an **app** 
will be considered as a distributed system.

> Single service itself also acts as an **app**. Here, we just refer as **app**
> to the specific business case of your need.



The services are created using **Service Lib**. 
The goal of **Service Lib** is to write re-usable solutions that will be
useful to another project with a minimal setup. Hence, why the services are
standalone applications .

To compose an app from the services in a structured way, the services are
divided into three categories.

### Independent
The first type of the services are **independent** services. Read it
as an independent software. Your app should have one independent service
that keeps the core logic of your application.

Independent services will rarely be shared. So the source
code could be private.

### Extension
The second type of the services are **extension** services. The extensions
are the solutions that could be re-used by multiple projects.

This is the core part that all makes the services as re-usable.

The extensions are allowed to be connected from the independent services.
And doesn't work with the users directly.

### Proxy
The third and last type of the services are **proxy** services. The proxy
acts as a switch between a user/service and a user/service. Depending on 
the proxy result the request will be forwarded or returned back to the client.

---
## Controller
Since the services are the units of distributed system, services
has to talk to each other. And services has to talk with the external world.

Therefore, each service acts as a server. The service mechanism 
transfers in or out some messages. 
This mechanism is implemented through controllers.

> Controller is an alias of server.
> 
> Controller term comes from the MPC pattern.

A service may have multiple controllers, at least one. 
The controllers receive the messages. Then controller is routing
the messages to the handlers. To find the right handler, the messages 
include the commands.

For optimization needs, there are different kinds of controllers.

### Replier
A **replier** controller handles a one request at a time. All incoming
requests are queued internally, until the current request is not executed.
When the request is executed, the controller returns the status to the callee.
Then replier will execute the next request in the queue.

> The requester will be waiting the response of the controller

### Router
A **router** controller handles many requests at a time. Upon the execution,
the router will reply the status back to the callee.

> The requester will be waiting the response of the controller

### Puller
A **puller** controller handles a one request at a time. All requests will be
queued internally. When the controller finishes the execution, it will
execute the next request in the queue.

Puller will not respond back to the callee about the status.

> The requester will not wait for the response of the controller.
> So this one is faster.

### Publisher
A **publisher** controller sends messages to the subscribers. It doesn't
receive the request from outside. But has internal **puller** controller
that the **publisher** is connected too. Any message coming into the **puller**
invokes mass message broadcast by the **publisher**.

> The subscriber waits for the controller, but doesn't request to the publisher.
> The invoker of the puller doesn't wait for the response of the publisher.

---

## Configuration
Any apps created by this module is loading environment
variables by default.

As well as it requires the *configuration* in yaml format.

### Env

You can set the Yaml file name as well as it's path
using the following environment variables:
```bash
SERVICE_CONFIG_NAME=service
SERVICE_CONFIG_PATH=.
```

By default, the service will look for `service.yml` in the `.` directory.

### Yaml

The configuration format is this:
```yaml
Services:
  - Type: Independent # or Proxy | Extension
    Name: 
    Instance: 
    Controllers:
      - Type: Replier # or Puller | Publisher | Router
        Name: "myApi"
        Instances:
          - Port: 2302
            Instance: 
    Proxies:
      - Name: "auth"
        Port: 8000

    Pipelines:
      - "auth->myApi"

    Extensions:
      - Name: "database"
        Port: 8002
```

At root, it has `Services` with at least one Service defined.
The service has the following parameters:

* Type which defines what kind of service it is. It could be `Independent`, `Proxy` or `Extension`.
* Name of the service. If you define multiple services, then their Type and Name should match.
* Instance is the unique identifier of this service. If you have multiple services, then it should have different instance.
* Controllers lists what kind of command handlers it has.
* Proxies lists what kind of proxies it has.
* Pipelines should have one or more proxy pipeline. The last name should name of the controller instance.
* Extensions lists the extensions that this service depends on. All these extensions are passed to the controllers.

The **controllers** are the command handlers. All incoming requests
from the users (whether it's through proxy or not) are handled by the controllers.
The parameters of the controllers:
* Type which defines what kind of controller it is. It could be Replier, Puller, Publisher or Router.
* Name of the controller to classify it.
* Instances describes the unique controllers of this type.

The controller instances have the following parameters:
* Instance is the unique id of the controller within all service
* Port where the controller exposes itself

Proxy has the following parameters:
* Name of the proxy
* Port where the proxy set too.

Extension has the following parameters:
* Name of the extension
* Port where the extension set too.

---

## Flags

The built service will have several flags and arguments.

* `--secure` enables authentication
* `--generate-config` rather than starting, it will create a default yaml configuration
* `--path=./a.yml` relative path of the generated configuration. The value should end with *.yml*.
* `--configuration=./b.yml` relative path of the configuration to use.

---

## Further reading

Once you understand the Services, go read further to create different kinds of services.

* [Proxy](./PROXY.md) to define proxy services.
* [Extension](./EXTENSION.md) to define extension services.
* [Independent](./INDEPENDENT.md) to define independent services.
