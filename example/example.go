package example

import (
	"fmt"
	"github.com/ahmetson/client-lib"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/handler-lib"
	"github.com/ahmetson/handler-lib/command"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/service-lib"
)

func main() {
	id := "example-app"
	example, err := service.New(id)

	if err != nil {
		log.Fatal("service.New", "id", id, "error", err)
	}

	h, _ := handler.Replier(example.Logger)

	var onHello = func(req message.Request, _ *log.Logger, _ ...*client.ClientSocket) message.Reply {
		name, _ := req.Parameters.GetString("name")

		param := key_value.Empty().Set("message", fmt.Sprintf("hello, %s", name))
		return req.Ok(param)
	}
	hello := command.NewRoute("hello", onHello)

	h.AddRoute(hello)

	example.AddController("game", h)

	if err := example.Prepare(); err != nil {
		log.Fatal("example.Prepare: %w", err)
	}
}
