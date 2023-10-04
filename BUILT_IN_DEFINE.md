# Built in definition

This page explains how to define the proxies within the code.
The compiled binary will include the proxies, therefore a proxy chain will be built in.

The built-in proxies are declared at the independent service.
Because of the self-orchestration, the destination service will act as a parent.
The parents are spawning the child services.

Since, proxies are services, they must be managed by the [context](https://github.com/ahmetson/dev-lib).
The context of the independent service takes care of the proxy preparation.

## Set Proxy Chain

Here is the code in the independent service to register a proxy chain.

```go
independentService.SetProxyChain(sources, proxies, destination)
```

This method sends the proxy chains to the proxy thread in the context.

### Url
The first argument is the service urls that can access to the proxies.
It's an optional. 
If it's not given, then anyone can access to the proxies.

The following code will work as well.
```go
independentService.SetProxyChain(proxies, destination)
```

The second argument is the proxy. 
It can be a single proxy or multiple proxies.
The last argument is the destination.

Let's explain the `Destination` argument first.

## Destination
Proxies can be applied to the whole service or certain handlers, or certain routes.
The route is the minimal destination.
When a proxy receives a request that's not in the route list, then it's simply forwarded.

The proxy destination is defined as a rule.
Because, services can come in and go, it can have multiple instances in the future.
Therefore, trying to put id of the future services is impossible. 
The rule makes a dynamic discovery.

> Using a pattern matching, the service generates the list of routes.
This allows a user to define a simple service or handler id.
Then, service will list the routes for the service or handler.

The rule is of the `Rule` structure instance.
This structure is defined in the `service` package of the `config-lib` module.
The `service` package has three methods to define a destination rule.

### Route destination

The `service.NewDestination(urls, categories, commands)` function defines the route rule.

Each of the arguments could be a scalar string or a list of strings:
The following two lines of the code are identical:

```go
service.NewDestination("url_1", "category_1", "command_1")
service.NewDestination([]string{"url_1"}, []string{"category_1"}, "command_1")
```

#### Urls
The first argument is the service url.
When defining a route destination, the url argument could be omitted.

```go
service.NewDestination("category_1", "command_1")
```

If the url is omitted, then the current independent service is considered as the `url`.

#### Categories
The second argument is the category of the handler.
It could be omitted or left empty.
If it's empty, then the routes will be considered in all the handlers.

The following code is valid:

```go
service.NewDestination("command_1")
```

> **Todo**
> 
> make the second argument as optional

#### Commands
The third argument is required.
It lists the route commands to apply the proxy.

### Handler destination
Listing all routes explicitly is tiresome. 
The other way to define routes is for the handlers.
The proxy will be applied to all the routes in the handlers.

The `service.NewHandlerDestination(urls, categories)` define the handler rule.

All arguments could be a scalar string or a list of strings.
The following two lines of code are equal:

```go
rule := service.NewHandlerDestination("url_1", "category_1")
rule := service.NewHandlerDestination([]string{"url_1"}, []string{"category_1"})
```

#### Urls
The first argument is the service url.
When defining a route destination, the url argument could be omitted.

```go
service.NewHandlerDestination("category_1")
```

If the url is omitted, then the current independent service is considered as the `url`.

#### Categories
The second argument is required.
It is the category of the handler.

#### Excluding commands
Sometimes when you define the handler rule, you want to exclude some routes there.
It's done by `service.Rule.Exclude(commands...)` method:

```go
rule.Exclude("command_1", "command_2", ...)
```

It's also possible to exclude by a rule:

```go
rule.ExcludeByRule(subRule)
```

In the `ExcludeByRule` the rule must have the list of commands or list of categories.

## Proxies
In the `independentService.SetProxyChain`, the second argument is the `Proxies`.

It's a list of the `Proxy` structure.
This structure is defined in the `service` package of the `config-lib` module.

The proxy is defined with the `NewProxy(id, url)` function from the `service` package.

The id is the unique id that the proxy will get.
The url is the address of the proxy source code.

## Source
The proxy receives the data from anyone who has access to the machine.
To narrow the access, we can set the sources.

The sources define the list of the url or at least a one. 
Then only the services of the url could access to the proxy.

> Checking the source is done by the security layer.

## Intercommunication algorithm
1. The independent service in the configuration preparation will ask for proxy chains that rule to this service.
2. The independent service will save the rules in the configuration.
3. The independent service creates a proxy with its own url and id using dep manager.
4. The proxy chain thread spans the last proxy with id, url, parent id.
5. The proxy will generate its configuration.
6. The proxy will ask for a list of routes from the parent then cache it.
7. The proxy will ask for a proxy chain by passing its id and url.
8. From the returned proxy chain, it will start the next proxy.
9. If it's the last proxy, then proxy will inform the parent as it's ready.

