package service

import (
	"fmt"
	"github.com/ahmetson/client-lib"
	clientConfig "github.com/ahmetson/client-lib/config"
	serviceConfig "github.com/ahmetson/config-lib/service"
	"github.com/ahmetson/datatype-lib/data_type/key_value"
	"github.com/ahmetson/datatype-lib/message"
	"github.com/ahmetson/handler-lib/base"
	handlerConfig "github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/route"
	"github.com/ahmetson/handler-lib/sync_replier"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/os-lib/arg"
	"github.com/ahmetson/os-lib/net"
	"github.com/ahmetson/os-lib/path"
	"github.com/ahmetson/os-lib/process"
	"github.com/ahmetson/service-lib/flag"
	"github.com/ahmetson/service-lib/manager"
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
type TestParentChildSuite struct {
	suite.Suite

	service    *Service // the manager to test
	currentDir string   // executable to store the binaries and source codes
	url        string   // dependency source code
	id         string   // the id of the dependency
	envPath    string
	handler    base.Interface
	logger     *log.Logger

	defaultHandleFunc route.HandleFunc0
	cmd1              string
	handlerCategory   string
}

func (test *TestParentChildSuite) createYaml(dir string, name string) {
	s := test.Require

	kv := key_value.New().Set("services", []interface{}{})

	marshalledConfig, err := yaml.Marshal(kv.Map())
	s().NoError(err)

	filePath := filepath.Join(dir, name+".yml")

	f, err := win.OpenFile(filePath, win.O_RDWR|win.O_CREATE|win.O_TRUNC, 0644)
	s().NoError(err)
	_, err = f.Write(marshalledConfig)
	s().NoError(err)

	s().NoError(f.Close())
}

func (test *TestParentChildSuite) deleteYaml(dir string, name string) {
	s := test.Require

	filePath := filepath.Join(dir, name+".yml")

	exist, err := path.FileExist(filePath)
	s().NoError(err)

	if !exist {
		return
	}

	s().NoError(win.Remove(filePath))
}

func (test *TestParentChildSuite) SetupTest() {
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
	_, err = file.WriteString(fmt.Sprintf("%s=%s\n%s=%s\n", flag.IdEnv, test.id, flag.UrlEnv, test.url))
	s().NoError(err, "failed to write the data into: "+test.envPath)
	err = file.Close()
	s().NoError(err, "delete the dump file: "+test.envPath)

	// handler
	syncReplier := sync_replier.New()
	test.defaultHandleFunc = func(req message.RequestInterface) message.ReplyInterface {
		return req.Ok(key_value.New())
	}
	test.cmd1 = "hello"
	s().NoError(syncReplier.Route(test.cmd1, test.defaultHandleFunc))
	test.handler = syncReplier

	test.logger, err = log.New("test", true)
	s().NoError(err)

	test.handlerCategory = "main"
	inprocConfig := handlerConfig.NewInternalHandler(handlerConfig.SyncReplierType, test.handlerCategory)
	test.handler.SetConfig(inprocConfig)
	s().NoError(test.handler.SetLogger(test.logger))
}

func (test *TestParentChildSuite) closeService() {
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

func (test *TestParentChildSuite) TearDownTest() {
	s := test.Suite.Require

	err := win.Remove(test.envPath)
	s().NoError(err, "delete the dump file: "+test.envPath)
}

func (test *TestParentChildSuite) newService() {
	s := test.Suite.Require

	win.Args = append(win.Args, arg.NewFlag(flag.IdFlag, test.id), arg.NewFlag(flag.UrlFlag, test.url))

	created, err := New()
	s().NoError(err)

	test.service = created
	test.service.SetHandler(test.handlerCategory, test.handler)
}

func (test *TestParentChildSuite) mainHandler() base.Interface {
	return test.service.Handlers["main"].(base.Interface)
}

func (test *TestParentChildSuite) externalClient(hConfig *handlerConfig.Handler) *client.Socket {
	s := test.Suite.Require

	// let's test that handler runs
	targetZmqType := handlerConfig.SocketType(hConfig.Type)
	externalConfig := clientConfig.New(test.service.url, hConfig.Id, hConfig.Port, targetZmqType)
	externalConfig.UrlFunc(clientConfig.Url)
	externalClient, err := client.New(externalConfig)
	s().NoError(err)

	return externalClient
}

func (test *TestParentChildSuite) managerClient() *manager.Client {
	s := test.Suite.Require

	createdConfig, err := test.service.ctx.Config().Service(test.id)
	s().NoError(err)
	managerConfig := createdConfig.Manager
	managerConfig.UrlFunc(clientConfig.Url)
	managerClient, err := manager.NewClient(managerConfig)
	s().NoError(err)

	return managerClient
}

// Test_10_Start test service start.
// It's the collection of all previous tested functions together
// The started service will make the handler and managers available
func (test *TestParentChildSuite) Test_10_Start() {
	s := test.Require

	proxyUrl := "github.com/ahmetson/service-lib/_test_services/proxy_1"
	proxyId := "proxy_1"
	proxyBinPath := path.BinPath(filepath.Join(".", "_test_services/proxy_1/bin"), "test6")
	proxyPort := 57397 // taken from ./_test_services/proxy_1/bin/app.yml

	used := net.IsPortUsed("localhost", proxyPort)
	if used {
		pid, err := process.PortToPid(proxyPort)
		if err != nil {
			panic(err)
		}
		proc, err := win.FindProcess(int(pid))
		if err != nil {
			panic(err)
		}

		err = proc.Kill()
		s().NoError(err)
	}

	win.Args = append(win.Args, arg.NewFlag(flag.IdFlag, test.id), arg.NewFlag(flag.UrlFlag, test.url))

	created, err := New()
	s().NoError(err)
	DeleteLastFlags(2)

	test.service = created
	test.service.SetHandler(test.handlerCategory, test.handler)

	test.service.ctx.SetService(test.service.id, test.service.url)
	err = test.service.ctx.StartDepManager()
	s().NoError(err)

	proxyConf := &serviceConfig.Proxy{
		Local: &serviceConfig.Local{
			LocalBin: proxyBinPath,
		},
		Id:       proxyId,
		Url:      proxyUrl,
		Category: "layer_1",
	}
	rule := serviceConfig.NewServiceDestination()
	err = test.service.SetProxyChain(proxyConf, rule)
	s().NoError(err)

	// No sources
	serviceConf, err := test.service.ctx.Config().Service(test.id)
	s().Error(err) // no service yet

	_, err = test.service.Start()
	s().NoError(err)

	// wait a bit for thread initialization
	time.Sleep(time.Second * 2)

	used = net.IsPortUsed("localhost", proxyPort)
	s().True(used)

	// Test that sources exist
	serviceConf, err = test.service.ctx.Config().Service(test.id)
	s().NoError(err)
	s().NotEmpty(serviceConf.Sources)

	// Make sure that manager is running
	managerClient := test.managerClient()
	err = managerClient.Close()
	s().NoError(err)

	// Wait a bit for closing
	time.Sleep(time.Second * 5)
	used = net.IsPortUsed("localhost", proxyPort)
	s().False(used)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestParentChild(t *testing.T) {
	suite.Run(t, new(TestParentChildSuite))
}
