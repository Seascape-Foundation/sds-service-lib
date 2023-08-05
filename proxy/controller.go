package proxy

import (
	"fmt"
	"github.com/ahmetson/service-lib/communication/message"
	service2 "github.com/ahmetson/service-lib/configuration/service"
	"github.com/ahmetson/service-lib/remote"
	zmq "github.com/pebbe/zmq4"

	"github.com/ahmetson/service-lib/log"
)

// ControllerName is the name of the proxy router that connects source and destination
const ControllerName = "proxy_controller"

// Url defines the proxy controller path
const Url = "inproc://" + ControllerName

// Destination is the client connected to the controller of the remote service.
// The Dealer is the Request from Router to the
// Reply Controller.
//
// The socket.ControllerType must be zmq.DEALER
type Destination struct {
	// Could be Remote or Inproc
	Configuration *service2.Controller
	// The client socket
	socket *zmq.Socket
}

// Controller is the internal process connecting source and destination.
type Controller struct {
	destination *Destination
	// type of the required destination
	requiredDestination service2.ControllerType
	logger              *log.Logger
	serviceUrl          string
}

// newController Returns the initiated Router with the service parameters that connects source and destination.
// along within the route, it will execute request handler and reply handler.
func newController(logger *log.Logger) *Controller {
	return &Controller{
		logger:              logger,
		destination:         nil,
		requiredDestination: service2.UnknownType,
	}
}

func (controller *Controller) RequireDestination(controllerType service2.ControllerType) {
	if controllerType != service2.UnknownType {
		controller.requiredDestination = controllerType
	}
}

// RegisterDestination Adds a new client that is connected to the Reply Controller.
// Verification of the service limit or service protocol type
// is handled on outside. As a result, it doesn't return
// any error.
// SDS Core can have unique command handlers.
func (controller *Controller) RegisterDestination(destinationConfig *service2.Controller, serviceUrl string) {
	controller.logger.Info("Adding client sockets that router will redirect", "destinationConfig", *destinationConfig)

	controller.serviceUrl = serviceUrl
	controller.destination = &Destination{Configuration: destinationConfig, socket: nil}
}

// Internal function that assigns the socket
// to the Clients.
//
// It's handled in this not in the socket.
// Because called from the router goroutine (go router.Run())
//
// If the Router creating thread calls
// then as thread-unsafely, will lead to the unexpected
// behaviors.
func (controller *Controller) setDestinationSocket() error {
	socket, err := zmq.NewSocket(zmq.DEALER)
	if err != nil {
		return fmt.Errorf("error creating socket: %w", err)
	}
	err = socket.Connect(remote.ClientUrl(controller.destination.Configuration.Instances[0].ControllerCategory, controller.destination.Configuration.Instances[0].Port))
	if err != nil {
		return fmt.Errorf("setup of dealer socket: %w", err)
	}

	controller.destination.socket = socket

	return nil
}

// Run the router (asynchronous zmq.REP) along with dealers (asynchronous zmq.REQ).
// The dealers are connected to the zmq.REP controllers.
//
// The router will redirect the message to the zmq.REP controllers using dealers.
//
// Note!!! If the request to this router comes from zmq.REQ client, then client
// should set the identity using zmq.SetIdentity().
//
// Format of the incoming message:
//
//		0 - bytes request id
//		1 - ""; empty delimiter
//		2 - string (app/parameter.Type) service name as a tag of dealer.
//	     to identify which dealer to use
//		3 - app/remote/message.Request the message that is redirected to the zmq.REP controller
//
// The request id is used to identify the client. Once the dealer gets the reply from zmq.REP controller
// the router will return it back to the client by request id.
//
// Example:
//
//	// route the msg[3] to the SDS Storage
//	msg := [0: "uid-123", 1: "", 2: "storage", 3: "{`command`: `smartcontract_get`, `parameters`: {}}"]
func (controller *Controller) Run() {
	if controller.destination == nil {
		controller.logger.Fatal("no destinations registered in the proxy", "hint", "call router.RegisterDestination()")
	}

	controller.logger.Info("setup the dealer sockets")
	//  Initialize poll set
	poller := zmq.NewPoller()

	// let's set the socket
	err := controller.setDestinationSocket()
	if err != nil {
		controller.logger.Fatal("setDestinationSocket", "destination instance", controller.destination.Configuration.Instances[0].Id)
	}
	poller.Add(controller.destination.socket, zmq.POLLIN)
	controller.logger.Info("dealers set up successfully")
	controller.logger.Info("setup router", "url", Url)

	frontend, _ := zmq.NewSocket(zmq.ROUTER)
	defer func() {
		err := frontend.Close()
		if err != nil {
			controller.logger.Fatal("failed to close the socket", "error", err)
		}
	}()
	err = frontend.Bind(Url)
	if err != nil {
		controller.logger.Fatal("zmq new router", "error", err)
	}
	hwm, _ := frontend.GetRcvhwm()
	controller.logger.Warn("high watermark from router", hwm)

	poller.Add(frontend, zmq.POLLIN)

	controller.logger.Info("The proxy controller waits for client requestMessages")

	//  Switch messages between sockets
	for {
		// The '-1' argument indicates waiting for the
		// infinite amount of time.
		sockets, err := poller.Poll(-1)
		if err != nil {
			controller.logger.Fatal("poller", "error", err)
		}
		for _, socket := range sockets {
			zmqSocket := socket.Socket
			// redirect to the dealer
			if zmqSocket == frontend {
				messages, err := zmqSocket.RecvMessage(0)
				request, parseErr := message.ParseRequest(messages[2:])
				if parseErr != nil {
					if err := replyErrorMessage(frontend, err, messages, message.Request{}); err != nil {
						controller.logger.Fatal("reply_error_message", "error", err)
					}
					continue
				}
				if err != nil {
					if err := replyErrorMessage(frontend, err, messages, request); err != nil {
						controller.logger.Fatal("reply_error_message", "error", err)
					}
					continue
				}

				if request.IsFirst() {
					request.SetUuid()
				}
				request.AddRequestStack(controller.serviceUrl, ControllerName, "instance01")

				controller.logger.Info("todo", "currently", "proxy redirects to the first destination", "todo", "need to direct through pipeline")
				client := controller.destination
				if client == nil {
					err := fmt.Errorf("'%s' dealer wasn't registered", messages[2])
					if err := replyErrorMessage(frontend, err, messages, request); err != nil {
						controller.logger.Fatal("reply_error_message", "error", err)
					}
					continue
				}

				requestString, _ := request.String()
				_, err = client.socket.SendMessage(messages[0], messages[1], requestString)
				if err != nil {
					controller.logger.Fatal("sendMessage failed", "message", requestString)
				}

				// end of handling requestMessages from source
				///////////////////////////////////////
			} else {
				messages, err := zmqSocket.RecvMessage(0)
				if err != nil {
					controller.logger.Fatal("failed to read message", "error", err)
				}
				reply, err := message.ParseReply(messages[2:])
				if err != nil {
					controller.logger.Fatal("ParseReply", "error", err)
				}
				if err := reply.SetStack(controller.serviceUrl, ControllerName, "instance01"); err != nil {
					controller.logger.Fatal("reply.SetStack", "error", err)
				}
				replyStr, _ := reply.String()
				if _, err := frontend.SendMessage(messages[0], messages[1], replyStr); err != nil {
					controller.logger.Fatal("frontend.SendMessage", "error", err)
				}
			}
		}
	}
}

// The router's error replier
func replyErrorMessage(socket *zmq.Socket, newErr error, messages []string, request message.Request) error {
	fail := request.Fail(newErr.Error())
	failString, _ := fail.String()

	_, err := socket.SendMessage(messages[0], messages[1], failString)
	if err != nil {
		return fmt.Errorf("failed to send back '%s': %w", failString, err)
	}

	return nil
}
