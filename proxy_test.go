package service

import (
	"fmt"
	clientConfig "github.com/ahmetson/client-lib/config"
	"github.com/ahmetson/datatype-lib/data_type/key_value"
	"github.com/ahmetson/datatype-lib/message"
	"github.com/ahmetson/handler-lib/base"
	handlerConfig "github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/route"
	"github.com/ahmetson/handler-lib/sync_replier"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/os-lib/arg"
	"github.com/ahmetson/os-lib/path"
	"github.com/ahmetson/service-lib/flag"
	"github.com/pebbe/zmq4"
	"github.com/stretchr/testify/suite"
	win "os"
	"path/filepath"
	"testing"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestProxySuite struct {
	suite.Suite

	parent     *Service // the manager to test
	currentDir string   // executable to store the binaries and source codes
	parentUrl  string   // dependency source code
	parentId   string   // the parentId of the dependency
	url        string
	id         string
	envPath    string
	handler    base.Interface
	logger     *log.Logger

	defaultHandleFunc route.HandleFunc0
	cmd1              string
	handlerCategory   string
}

func (test *TestProxySuite) SetupTest() {
	s := test.Suite.Require

	currentDir, err := path.CurrentDir()
	s().NoError(err)
	test.currentDir = currentDir

	// A valid source code that we want to download
	test.parentUrl = "github.com/ahmetson/parent-lib"
	test.parentId = "service_1"
	test.url = "github.com/ahmetson/proxy-lib"
	test.id = "proxy_1"

	test.envPath = filepath.Join(currentDir, ".test.env")

	file, err := win.Create(test.envPath)
	s().NoError(err)
	_, err = file.WriteString(fmt.Sprintf("%s=%s\n%s=%s\n", flag.IdEnv, test.parentId, flag.UrlEnv, test.parentUrl))
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

func (test *TestProxySuite) TearDownTest() {
	s := test.Suite.Require

	err := win.Remove(test.envPath)
	s().NoError(err, "delete the dump file: "+test.envPath)
}

// Test_10_NewProxy tests NewProxy
func (test *TestProxySuite) Test_10_NewProxy() {
	s := test.Suite.Require

	// Creating a proxy with the valid flags must succeed
	parentClient := clientConfig.New(test.parentUrl, test.parentId, 6000, zmq4.REP)
	parentKv, err := key_value.NewFromInterface(parentClient)
	s().NoError(err)
	parentStr := parentKv.String()
	win.Args = append(win.Args,
		arg.NewFlag(flag.IdFlag, test.id),
		arg.NewFlag(flag.UrlFlag, test.url),
		arg.NewFlag(flag.ParentFlag, parentStr),
	)

	proxy, err := NewProxy()
	s().NoError(err)

	DeleteLastFlags(3)

	s().NoError(proxy.ctx.Close())
}

//// The started parent will make the handler and managers available
//func (test *TestProxySuite) Test_17_Start() {
//	s := test.Require
//
//	// first start a parent
//	// then start an auxiliary
//	// set to the proxy chain
//
//	test.newService()
//
//	_, err := test.parent.Start()
//	s().NoError(err)
//
//	// wait a bit for thread initialization
//	time.Sleep(time.Millisecond * 100)
//
//	// let's test that handler runs
//	mainHandler := test.mainHandler()
//	externalClient := test.externalClient(mainHandler.Config())
//
//	// Make sure that handlers are running
//	req := message.Request{
//		Command:    "hello",
//		Parameters: key_value.New(),
//	}
//	reply, err := externalClient.Request(&req)
//	s().NoError(err)
//	s().True(reply.IsOK())
//
//	// Make sure that manager is running
//	managerClient := test.managerClient()
//	req = message.Request{
//		Command:    "heartbeat",
//		Parameters: key_value.New(),
//	}
//	reply, err = managerClient.Request(&req)
//	s().NoError(err)
//	s().True(reply.IsOK())
//
//	// clean out
//	// we don't close the handler here by calling mainHandler.Close.
//	//
//	// the parent manager must close all handlers.
//	s().NoError(test.parent.manager.Close())
//
//	// since we closed by manager, the cleaning-out by test suite is not necessary.
//	test.parent = nil
//	win.Args = win.Args[:len(win.Args)-2]
//}
//
//// Test_18_Service_unitsByRouteRule tests the counting units by route rule
//func (test *TestProxySuite) Test_18_Service_unitsByRouteRule() {
//	s := test.Require
//
//	cmd2 := "cmd_2"
//	category2 := "category_2"
//
//	// the SetupTest adds "main" category handler with "hello" command
//	test.newService()
//	rule := serviceConfig.NewDestination(test.parent.url, test.handlerCategory, test.cmd1)
//	units := test.parent.unitsByRouteRule(rule)
//	s().Len(units, 1)
//
//	// if the rule has a command that doesn't exist in the parent, it's skipped
//	rule.Commands = []string{test.cmd1, cmd2}
//	units = test.parent.unitsByRouteRule(rule)
//	s().Len(units, 1)
//
//	// suppose the handler has both commands; then units must return both
//	err := test.handler.Route(cmd2, test.defaultHandleFunc)
//	s().NoError(err)
//	test.parent.SetHandler(test.handlerCategory, test.handler)
//
//	units = test.parent.unitsByRouteRule(rule)
//	s().Len(units, 2)
//
//	// let's say; we have two handlers, in this case search for commands in all categories
//	syncReplier := sync_replier.New()
//	s().NoError(syncReplier.Route(test.cmd1, test.defaultHandleFunc))
//	inprocConfig := handlerConfig.NewInternalHandler(handlerConfig.SyncReplierType, category2)
//	syncReplier.SetConfig(inprocConfig)
//	s().NoError(syncReplier.SetLogger(test.logger))
//	test.parent.SetHandler(category2, syncReplier)
//	rule.Categories = []string{test.handlerCategory, category2}
//
//	units = test.parent.unitsByRouteRule(rule)
//	s().Len(units, 3)
//
//	// clean out
//	test.closeService()
//}
//
//// Test_19_Service_unitsByHandlerRule tests the counting units by handler rule
//func (test *TestProxySuite) Test_19_Service_unitsByHandlerRule() {
//	s := test.Require
//
//	cmd2 := "cmd_2"
//	category2 := "category_2"
//
//	// the SetupTest adds "main" category handler with "hello" command
//	test.newService()
//	rule := serviceConfig.NewDestination(test.parent.url, test.handlerCategory, test.cmd1)
//	units := test.parent.unitsByHandlerRule(rule)
//	s().Len(units, 1)
//
//	// if the rule has a command that doesn't exist in the parent, it's skipped
//	rule.Commands = []string{test.cmd1, cmd2}
//	units = test.parent.unitsByHandlerRule(rule)
//	s().Len(units, 1)
//
//	// The above code is identical too Handler Rule.
//	rule = serviceConfig.NewHandlerDestination(test.parent.url, test.handlerCategory)
//	units = test.parent.unitsByHandlerRule(rule)
//	s().Len(units, 1)
//
//	// suppose the handler has both commands; then units must return both
//	err := test.handler.Route(cmd2, test.defaultHandleFunc)
//	s().NoError(err)
//	test.parent.SetHandler(test.handlerCategory, test.handler)
//
//	units = test.parent.unitsByHandlerRule(rule)
//	s().Len(units, 2)
//
//	// let's say; we have two handlers, in this case search for commands in all categories
//	syncReplier := sync_replier.New()
//	s().NoError(syncReplier.Route(test.cmd1, test.defaultHandleFunc))
//	inprocConfig := handlerConfig.NewInternalHandler(handlerConfig.SyncReplierType, category2)
//	syncReplier.SetConfig(inprocConfig)
//	s().NoError(syncReplier.SetLogger(test.logger))
//	test.parent.SetHandler(category2, syncReplier)
//
//	rule = serviceConfig.NewHandlerDestination(test.parent.url, []string{test.handlerCategory, category2})
//
//	units = test.parent.unitsByHandlerRule(rule)
//	s().Len(units, 3)
//
//	// Excluding the command must not return them as a unit
//	rule.ExcludeCommands(test.cmd1)
//	units = test.parent.unitsByHandlerRule(rule)
//	s().Len(units, 1) // the test.cmd1 exists in two handlers, cmd2 from first handler must be returned
//
//	rule.ExcludeCommands(cmd2)
//	units = test.parent.unitsByHandlerRule(rule)
//	s().Len(units, 0) // all commands are excluded.
//
//	// clean out
//	test.closeService()
//}
//
//// Test_20_Service_unitsByServiceRule tests the counting units by parent rule
//func (test *TestProxySuite) Test_20_Service_unitsByServiceRule() {
//	s := test.Require
//
//	cmd2 := "cmd_2"
//	category2 := "category_2"
//
//	// the SetupTest adds "main" category handler with "hello" command
//	test.newService()
//	rule := serviceConfig.NewServiceDestination(test.parent.url)
//	units := test.parent.unitsByServiceRule(rule)
//	s().Len(units, 1)
//
//	// suppose the handler has both commands; then units must return both
//	err := test.handler.Route(cmd2, test.defaultHandleFunc)
//	s().NoError(err)
//	test.parent.SetHandler(test.handlerCategory, test.handler)
//
//	units = test.parent.unitsByServiceRule(rule)
//	s().Len(units, 2)
//
//	// let's say; we have two handlers, in this case search for commands in all categories
//	syncReplier := sync_replier.New()
//	s().NoError(syncReplier.Route(test.cmd1, test.defaultHandleFunc))
//	inprocConfig := handlerConfig.NewInternalHandler(handlerConfig.SyncReplierType, category2)
//	syncReplier.SetConfig(inprocConfig)
//	s().NoError(syncReplier.SetLogger(test.logger))
//	test.parent.SetHandler(category2, syncReplier)
//
//	units = test.parent.unitsByServiceRule(rule)
//	s().Len(units, 3)
//
//	// clean out
//	test.closeService()
//}

func TestProxy(t *testing.T) {
	suite.Run(t, new(TestProxySuite))
}
