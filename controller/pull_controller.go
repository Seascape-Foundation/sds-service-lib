/*
Package controller defines the data type of the various server sockets.

Using the controller package, you can turn application to the servers.

The following types of controllers are supported:

  - Pull creates a puller for the service. Puller enables the inputs, but doesn't respond back.
  - Reply creates a replier for the service. Reply executes the messages and replies back to the caller.
  - Router creates a proxy/broker for the service. Router forwards the requests to other Router/Reply or Pull
*/
package controller

import (
	"fmt"

	"github.com/Seascape-Foundation/sds-service-lib/service/log"
	"github.com/Seascape-Foundation/sds-service-lib/service/parameter"

	zmq "github.com/pebbe/zmq4"
)

// NewPull creates a pull controller for the service.
func NewPull(s *parameter.Service, logger log.Logger) (*Controller, error) {
	if !s.IsThis() && !s.IsInproc() {
		return nil, fmt.Errorf("service should be limited to parameter.THIS or inproc type")
	}
	controller_logger, err := logger.Child("controller", "type", "pull", "service_name", s.Name, "inproc", s.IsInproc())
	if err != nil {
		return nil, fmt.Errorf("error creating child logger: %w", err)
	}

	// Socket to talk to clients
	socket, err := zmq.NewSocket(zmq.PULL)
	if err != nil {
		return nil, fmt.Errorf("zmq.NewSocket: %w", err)
	}

	return &Controller{
		socket:      socket,
		service:     s,
		logger:      controller_logger,
		socket_type: zmq.PULL,
	}, nil
}
