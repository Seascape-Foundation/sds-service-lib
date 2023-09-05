# Tutorial

Let's start with the hello world:

```go
package main

import (
	"github.com/ahmetson/client-lib"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/service-lib"
)

func onHello(req message.Request, _ *log.Logger, _ ...*client.ClientSocket) message.Reply {
	repl := key_value.Empty().Set("message", "hello world")

	return req.Ok(repl)
}

func main() {
	appName := "sample-app"
	app, _ := service.New(appName)

	route := command.NewRoute("hello", onHello)

	server, _ := handler.Replier(app.Logger)
	server.AddRoute(route)
	app.AddController("server", server)
	app.RequireProxy("github.com/ahmetson/http-proxy")
	app.Prepare()
	app.Run()
}
```

You see it's twenty lines of code. But that hides a lot of stuff of this
powerful tool.

We always create a service with an id.
Once the service is created, we always add the handler.
After the handler, we add the proxy which is optional.
Then we prepare it.
And then we run our service.

The handler is adding a route which is the command and a handler of the command.

Compile, and run it. It should work fine.

----

# Adding extension

Create a new extension. 
Then add it as:

```go
	app, _ := service.New("counter")
	app.AddExtension("github.com/ahmetson/counter-extension")
	
	app.Prepare()
	app.Run()
```

---

# Updating the services
Now, instead of recompiling it every time, what we will do is
to enter into the meta environment:

```shell
./bin/app --meta
```

This meta, allows you to enter into the software, but to look at it
from the beyond. Even when the software is running.

And here, I have some proxies that I want to add:

```shell
"ftp-proxy"
"auth-proxy"
"plain-proxy"
"viewer-proxy"
```

If I run inside the meta:

```shell
add proxy -g "ftp" "auth" "plain" "viewer"; proxy -g install; proxy -g prepare; proxy -g run
```

Then it will do some magic:
Here is the pipeline:

**ftp -> auth -> plain -> viewer -> my-app**

The application understood the orders in which they should be set.

I can also update the code.

```shell
update proxy -g
```

It will try to fetch, build and set them. And if the proxies don't have the 
tests, then they will fail. During the update, the proxy will change do it
in a live mode.

How about, creating a new instance?

```shell
clone my-app
```

If we had:

```my-app/server``` handler.
Then, it will turn itself into:

```
                    /-> my-app/server
load-balancer-proxy
                    \-> my-app-1/server
```

I can change that proxy:

```shell
replace load-balancer-proxy backup-proxy
```

```
                    /-> my-app/server (primary)
backup-proxy
                    \-> my-app-1/server (replication)
```

But, most importantly, the service is doing it automatically.

--- 

# Auto-management

Going back to our previous example, let's add few more routes:

```go
add := command.NewRoute("calc_add", nil)
sub := command.NewRoute("calc_sub", nil)
```

We can build and run it. If our application is already running, 
it will load this version as a clone, stop the previous one, 
and then redirect all requests to the updated version.

Now, if we will go and see what our code is doing:

```go
cs my-app
show handlers
```

It will show to us:

```go
calc (calculator server)
counter (update or return the number)
```

Since, its decoupled.

