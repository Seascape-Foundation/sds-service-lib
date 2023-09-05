# Tutorial

Let's start with the classic *"Hello World"*:

```go
package service

import (
	"github.com/ahmetson/client-lib"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/dev-lib/base/config"
	"github.com/ahmetson/handler-lib/replier"
	"github.com/ahmetson/service-lib"
)

func onHello(req message.Request) message.Reply {
	repl := key_value.Empty().
		Set("message", "hello world")

	return req.Ok(repl)
}

func main() {
	serviceId := "sample-app"
	serviceUrl := "github.com/ahmetson/url"
	app, _ := service.New(serviceId, serviceUrl, config.DevContext)

	server := replier.New()
	server.Route("hello", onHello)

	app.AddController("main", server)
	app.RequireProxy("github.com/ahmetson/http-proxy")
	app.Run()
}
```

The code is ten lines of code. 
It creates a service in the `config.DevContext`.
Then it adds a server and a proxy into the service.

Lastly, the server has a route which calls `onHello` function for `hello` command.

Compile, and run it. It should work fine.
The service will download the proxy from the URL that you provided.

----

# Adding extension

Create a new extension. 
Then add it as:

```go
package service

import (
	"fmt"
	"github.com/ahmetson/client-lib"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/dev-lib/base/config"
	"github.com/ahmetson/handler-lib/replier"
	"github.com/ahmetson/service-lib"
)

func onHelloName(req message.Request, db *client.Socket) message.Reply {
	next := req.Next("get-name", req.Parameters)
	reply, err := db.Request(next)
	if err != nil {
		return req.Fail(fmt.Sprintf("get-name failed: %v", err))
	}

	if !reply.IsOk() {
		return req.Fail(fmt.Sprintf("reply.Message: %s", reply.Message))
	}

	return req.Ok(reply.Parameters)
}

func main() {
	app, _ := service.New("app", "github.com/ahmetson/app", config.DevContext)

	server := replier.New()
	ext := "github.com/ahmetson/mysql-extension"
	server.Route("hello-name", onHelloName, ext)
	
	app.AddController("main", server)
	app.RequireProxy("github.com/ahmetson/http-proxy")
	app.Run()
}

```

Note, that the `hello-name` handler function requires an extension.
We pass this extension by its url.

In total, our app depends on two binaries: `http-proxy` and `mysql-extension`.
The `mysql-extension` itself depends on the `mysql` as well. 
But that's the problem of the extension. 
In our service, we don't worry about anything else.

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

