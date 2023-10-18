# Proxy
The proxies are special services that operate before independent services.
If the user request passes the proxy handling, then it's forwarded.

Proxies are organized as a chain.
A user request comes to proxy A. 
If it passes the condition, then proxy A forwards to proxy B.
If the request passes the condition, then it's forwarded to the independent service.

**Proxies either reject or forward the messages.
So, don't set the business logic in the proxies**.

Examples of the proxies are 
* validation, 
* authentication, 
* authorization, 
* load-balancing,
* and many more...

The proxies are started by the independent service.
But it's possible to manage the parent from the proxy itself.

Think, independent service as the parent and proxy as a child.
When a parent gets old, a child has to look after it.

Such proxies are classified in the *reliability* category.

But what is the proxy category?

## Category
As said earlier, proxies can be set as a chain.
When there is a proxy chain, there must be some order of the proxies.
To easily set the valid order, we categorize the proxies by their purpose.

This order helps us to invalid requests as soon as possible without
over laying computer.

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

The `service.NewProxy(id: string, url: string)` returns the pointer to `service.Proxy`.

The proxy structure is:

```typescript
type Proxy = {
    Url: string;
    Id: string;
    Category?: string; 
}
```

The destination structure defined as `service.Rule` in the `config-lib` module:

```typescript
type Rule ={
	Urls:                string[] // the service url 
	Categories:          string[] // handler category 
	Commands:            string[] // route name 
	ExcludedCommands:    string[]
}
```

The sources are defined as a list of strings: `string[]`.

The proxy chains are defined as `service.ProxyChain` in the `config-lib` module.

```typescript
type ProxyChain = {
    Sources?: string[]
    Proxies: Proxy[] // pointer
    Destination: Rule // pointer
}
```

> The category is not implemented yet.

> **todo**
>
> Generate the id from proxy-chain, destination.

---

## Execution
The above code described the proxy definition in the `config-lib`.
Now, it's the time to define the proxy within the `proxy-lib` itself.

# Handlers

By design, the proxies are intended to be invisible for the requester.
If a user connects to the independent service through the proxy,
then proxy is considered as the independent service itself.

Remember that in independent services we define the routes
and group them by the handlers.
In the proxies, we define pass function only.
The routes and handlers are fetched from the destination.
And the `Rule` parameter filters them.

**As a developer, you can not set a route or a handler.**

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

---

---
need to add an easy way to see the status of the application
Is it through meta
Is it through logs with the progress? For example:
service started
service started proxy
proxy started
app is ready.
Is it through the dashboard
