package service

import (
	"fmt"
	"github.com/ahmetson/client-lib"
	clientConfig "github.com/ahmetson/client-lib/config"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/common-lib/message"
	"github.com/ahmetson/handler-lib/base"
	handlerConfig "github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/manager_client"
	"github.com/ahmetson/handler-lib/sync_replier"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/os-lib/arg"
	"github.com/ahmetson/os-lib/path"
	"github.com/ahmetson/service-lib/config"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
	win "os"
	"path/filepath"
	"testing"
	"time"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestServiceSuite struct {
	suite.Suite

	service    *Service // the manager to test
	currentDir string   // executable to store the binaries and source codes
	url        string   // dependency source code
	id         string   // the id of the dependency
	envPath    string
	handler    base.Interface
	logger     *log.Logger
}

func (test *TestServiceSuite) createYaml(dir string, name string) {
	s := test.Require

	kv := key_value.New().Set("services", []interface{}{})

	serviceConfig, err := yaml.Marshal(kv.Map())
	s().NoError(err)

	filePath := filepath.Join(dir, name+".yml")

	f, err := win.OpenFile(filePath, win.O_RDWR|win.O_CREATE|win.O_TRUNC, 0644)
	s().NoError(err)
	_, err = f.Write(serviceConfig)
	s().NoError(err)

	s().NoError(f.Close())
}

func (test *TestServiceSuite) deleteYaml(dir string, name string) {
	s := test.Require

	filePath := filepath.Join(dir, name+".yml")

	exist, err := path.FileExist(filePath)
	s().NoError(err)

	if !exist {
		return
	}

	s().NoError(win.Remove(filePath))
}

func (test *TestServiceSuite) SetupTest() {
	s := test.Suite.Require

	currentDir, err := path.CurrentDir()
	s().NoError(err)
	test.currentDir = currentDir

	// A valid source code that we want to download
	test.url = "github.com/ahmetson/service-lib"
	test.id = "service_1"

	test.envPath = filepath.Join(currentDir, ".test.env")

	file, err := win.Create(test.envPath)
	s().NoError(err)
	_, err = file.WriteString(fmt.Sprintf("%s=%s\n%s=%s\n", config.IdEnv, test.id, config.UrlEnv, test.url))
	s().NoError(err, "failed to write the data into: "+test.envPath)
	err = file.Close()
	s().NoError(err, "delete the dump file: "+test.envPath)

	// handler
	syncReplier := sync_replier.New()
	onHello := func(req message.RequestInterface) message.ReplyInterface {
		return req.Ok(key_value.New())
	}
	s().NoError(syncReplier.Route("hello", onHello))
	test.handler = syncReplier

	test.logger, err = log.New("test", true)
	s().NoError(err)
}

func (test *TestServiceSuite) closeService() {
	s := test.Suite.Require
	if test.service != nil {
		s().NoError(test.service.ctx.Close())

		test.service = nil

		win.Args = win.Args[:len(win.Args)-2]

		// Wait a bit for closing the threads
		time.Sleep(time.Second)
	}

	test.deleteYaml(test.currentDir, "app")
}

func (test *TestServiceSuite) TearDownTest() {
	s := test.Suite.Require

	err := win.Remove(test.envPath)
	s().NoError(err, "delete the dump file: "+test.envPath)
}

func (test *TestServiceSuite) newService() {
	s := test.Suite.Require

	win.Args = append(win.Args, arg.NewFlag(config.IdFlag, test.id), arg.NewFlag(config.UrlFlag, test.url))

	created, err := New()
	s().NoError(err)

	test.service = created
	test.service.SetHandler("main", test.handler)
}

func (test *TestServiceSuite) mainHandler() base.Interface {
	return test.service.Handlers["main"].(base.Interface)
}

func (test *TestServiceSuite) externalClient(hConfig *handlerConfig.Handler) *client.Socket {
	s := test.Suite.Require

	// let's test that handler runs
	targetZmqType := handlerConfig.SocketType(hConfig.Type)
	externalConfig := clientConfig.New(test.service.url, hConfig.Id, hConfig.Port, targetZmqType)
	externalConfig.UrlFunc(clientConfig.Url)
	externalClient, err := client.New(externalConfig)
	s().NoError(err)

	return externalClient
}

func (test *TestServiceSuite) managerClient() *client.Socket {
	s := test.Suite.Require

	managerConfig := test.service.config.Manager
	managerConfig.UrlFunc(clientConfig.Url)
	managerClient, err := client.New(managerConfig)
	s().NoError(err)

	return managerClient
}

// Test_10_New new service by flag or environment variable
func (test *TestServiceSuite) Test_10_New() {
	s := test.Suite.Require

	// creating a new service must fail since
	// no flag or environment variable to identify service
	_, err := New()
	s().Error(err)

	// Wait a bit for closing context threads
	time.Sleep(time.Millisecond * 100)

	// Pass a flag
	idFlag := arg.NewFlag(config.IdFlag, test.id)
	urlFlag := arg.NewFlag(config.UrlFlag, test.url)
	win.Args = append(win.Args, idFlag, urlFlag)

	independent, err := New()
	s().NoError(err)

	// Clean out the os args
	win.Args = win.Args[:len(win.Args)-2]

	// remove the created service.
	// to re-create the service, we must close the context.
	s().NoError(independent.ctx.Close())
	// wait a bit for closing context threads
	time.Sleep(time.Millisecond * 500)

	// try to load from the environment variable parameters
	win.Args = append(win.Args, test.envPath)

	independent, err = New()
	s().NoError(err)

	// Wait a bit for context initialization
	time.Sleep(time.Millisecond * 100)

	// remove the environment variable from arguments
	win.Args = win.Args[:len(win.Args)-1]

	// remove the created service, and try from environment variable
	s().NoError(independent.ctx.Close())

	// Wait a bit for closing context threads
	time.Sleep(time.Millisecond * 100)
}

// Test_11_generateConfig creates a configuration and sets it in the service
func (test *TestServiceSuite) Test_11_generateConfig() {
	s := test.Suite.Require

	test.newService()

	_, err := test.service.generateConfig()
	s().NoError(err)

	test.closeService()
}

// Test_12_lintConfig loads the configuration of the service and sets it
func (test *TestServiceSuite) Test_12_lintConfig() {
	s := test.Suite.Require

	test.newService()

	_, err := test.service.generateConfig()
	s().NoError(err)

	s().NoError(test.service.lintConfig())

	test.closeService()
}

// Test_13_prepareConfig is calling lint config since the configuration exists in the context.
func (test *TestServiceSuite) Test_13_prepareConfig() {
	s := test.Suite.Require

	test.newService()

	// by default no configuration
	s().Nil(test.service.config)

	// It should call the test.service.lintConfig
	s().NoError(test.service.prepareConfig())

	// Config must be set
	s().NotNil(test.service.config)

	test.closeService()
}

// Test_14_manager tests the creation of the manager and linting it with the handler.
func (test *TestServiceSuite) Test_14_manager() {
	s := test.Suite.Require

	test.newService()
	s().NoError(test.service.prepareConfig())

	s().NoError(test.service.newManager())

	handler := test.service.Handlers["main"].(base.Interface)
	err := test.service.setHandlerClient(handler)
	s().NoError(err)

	test.closeService()
}

// Test_15_handler tests setup and start of the handler
func (test *TestServiceSuite) Test_15_handler() {
	s := test.Suite.Require

	test.newService()
	s().NoError(test.service.prepareConfig())

	s().NoError(test.service.newManager())

	handler := test.mainHandler()
	s().NoError(test.service.startHandler(handler))

	// wait a bit until the handler is initialized
	time.Sleep(time.Millisecond * 100)

	// let's test that handler runs
	externalClient := test.externalClient(handler.Config())

	// request the handler
	req := message.Request{
		Command:    "hello",
		Parameters: key_value.New(),
	}
	reply, err := externalClient.Request(&req)
	s().NoError(err)
	s().True(reply.IsOK())

	// close the handler
	handlerManager, err := manager_client.New(handler.Config())
	s().NoError(err)
	s().NoError(handlerManager.Close())
	s().NoError(externalClient.Close())

	test.closeService()
}

// Test_16_managerRequest tests the start of the manager and closing it by a command
func (test *TestServiceSuite) Test_16_managerRequest() {
	s := test.Suite.Require

	test.newService()
	s().NoError(test.service.prepareConfig())

	s().NoError(test.service.newManager())

	handler := test.service.Handlers["main"].(base.Interface)
	err := test.service.setHandlerClient(handler)
	s().NoError(err)

	s().NoError(test.service.startHandler(handler))

	s().NoError(test.service.manager.Start())

	// wait a bit until the handler and manager are initialized
	time.Sleep(time.Millisecond * 100)

	// test sending a command to the manager
	externalConfig := test.service.config.Manager
	externalConfig.UrlFunc(clientConfig.Url)
	externalClient, err := client.New(externalConfig)
	s().NoError(err)

	req := message.Request{
		Command:    "close",
		Parameters: key_value.New(),
	}
	err = externalClient.Submit(&req)
	s().NoError(err)

	// Wait a bit for closing service threads
	time.Sleep(time.Millisecond * 100)

	// make sure that context is not running
	s().False(test.service.ctx.Running())

	// clean out
	test.service = nil
	win.Args = win.Args[:len(win.Args)-2]
}

// Test_17_Start test service start.
// It's the collection of all previous tested functions together
// The started service will make the handler and managers available
func (test *TestServiceSuite) Test_17_Start() {
	s := test.Require

	test.newService()

	_, err := test.service.Start()
	s().NoError(err)

	// wait a bit for thread initialization
	time.Sleep(time.Millisecond * 100)

	// let's test that handler runs
	mainHandler := test.mainHandler()
	externalClient := test.externalClient(mainHandler.Config())

	// Make sure that handlers are running
	req := message.Request{
		Command:    "hello",
		Parameters: key_value.New(),
	}
	reply, err := externalClient.Request(&req)
	s().NoError(err)
	s().True(reply.IsOK())

	// Make sure that manager is running
	managerClient := test.managerClient()
	req = message.Request{
		Command:    "heartbeat",
		Parameters: key_value.New(),
	}
	reply, err = managerClient.Request(&req)
	s().NoError(err)
	s().True(reply.IsOK())

	// clean out
	// we don't close the handler here by calling mainHandler.Close.
	//
	// the service manager must close all handlers.
	s().NoError(test.service.manager.Close())

	// since we closed by manager, the cleaning-out by test suite not necessary.
	test.service = nil
	win.Args = win.Args[:len(win.Args)-2]
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestService(t *testing.T) {
	suite.Run(t, new(TestServiceSuite))
}
