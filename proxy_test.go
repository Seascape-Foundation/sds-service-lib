package service

import (
	"fmt"
	clientConfig "github.com/ahmetson/client-lib/config"
	"github.com/ahmetson/config-lib/app"
	"github.com/ahmetson/config-lib/service"
	"github.com/ahmetson/datatype-lib/data_type/key_value"
	"github.com/ahmetson/datatype-lib/message"
	"github.com/ahmetson/handler-lib/base"
	handlerConfig "github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/manager_client"
	"github.com/ahmetson/handler-lib/route"
	"github.com/ahmetson/handler-lib/sync_replier"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/os-lib/arg"
	"github.com/ahmetson/os-lib/path"
	"github.com/ahmetson/service-lib/flag"
	"github.com/ahmetson/service-lib/manager"
	"github.com/stretchr/testify/suite"
	win "os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestProxySuite struct {
	suite.Suite

	parent            *Service // the manager to test
	parentUrl         string   // dependency source code
	parentId          string   // the parentId of the dependency
	parentLocalBin    string
	parentConfig      *app.App
	parentProxyChains []*service.ProxyChain
	url               string
	id                string
	handler           base.Interface
	logger            *log.Logger

	defaultHandleFunc route.HandleFunc0
	cmd1              string
	handlerCategory   string
}

// SetupTest prepares the following:
//
//   - current exec directory
//   - parent id, url and proxy id, url
//   - handler for a parent, along with TestProxySuite.cmd1 route.
func (test *TestProxySuite) SetupTest() {
	s := test.Suite.Require

	// A valid source code that we want to download
	test.parentUrl = "github.com/ahmetson/today-do"
	test.parentId = "todaydo"
	test.parentLocalBin = path.BinPath(filepath.Join("./_test_services/proxy_parent/backend/bin"), "test")
	test.parentConfig = app.New()
	test.url = "github.com/ahmetson/proxy-lib"
	test.id = "proxy_1"

	// load the parent configuration
	parentConfigPath := filepath.Join("./_test_services/proxy_parent/backend/bin/app.yml")
	err := app.Read(parentConfigPath, test.parentConfig)
	s().NoError(err)

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
	inprocConfig, err := handlerConfig.NewHandler(handlerConfig.SyncReplierType, test.handlerCategory)
	s().NoError(err)
	test.handler.SetConfig(inprocConfig)
	s().NoError(test.handler.SetLogger(test.logger))
}

func (test *TestProxySuite) TearDownTest() {
	//s := test.Suite.Require
}

func (test *TestProxySuite) mockedProxyChainsByLastProxy(req message.RequestInterface) message.ReplyInterface {
	fmt.Printf("test.mockedProxyChainsByLastProxy entered\n")

	id, err := req.RouteParameters().StringValue("id")
	fmt.Printf("test.mockedProxyChainsByLastProxy params, id='%s', err: %v\n", id, err)
	if err != nil {
		return req.Fail("id parameter is missing")
	}
	proxyChains := make([]key_value.KeyValue, 0, 1)

	if test.id != id || len(test.parentProxyChains) == 0 {
		return req.Ok(key_value.New().Set("proxy_chains", proxyChains))
	}

	fmt.Printf("test.mockedProxyChainsByLastProxy convert ProxyChain to KeyValue\n")

	for i := range test.parentProxyChains {
		kv, err := key_value.NewFromInterface(test.parentProxyChains[i])
		if err != nil {
			return req.Fail(fmt.Sprintf("test.parentProxyChains[%d]: %v", i, err))
		}
		proxyChains = append(proxyChains, kv)
	}
	fmt.Printf("test.mockedProxyChainsByLastProxy proxy chains to return: %v\n", proxyChains)

	return req.Ok(key_value.New().Set("proxy_chains", proxyChains))
}

func (test *TestProxySuite) mockedHandlersByRuleEmpty(req message.RequestInterface) message.ReplyInterface {
	fmt.Printf("test.mockedHandlersByRuleEmpty entered\n")

	kvs := make([]key_value.KeyValue, 0)

	return req.Ok(key_value.New().Set("handler_configs", kvs))
}

func (test *TestProxySuite) mockedHandlersByRuleTriggers(req message.RequestInterface) message.ReplyInterface {
	s := test.Require

	fmt.Printf("test.mockedHandlersByRuleTriggers entered\n")

	kvs := make([]key_value.KeyValue, 2)

	conf1, err := handlerConfig.NewHandler(handlerConfig.SyncReplierType, "trigger_1")
	s().NoError(err)
	trigger1, err := handlerConfig.TriggerAble(conf1, handlerConfig.PublisherType)
	s().NoError(err)
	conf2, err := handlerConfig.NewHandler(handlerConfig.ReplierType, "trigger_2")
	s().NoError(err)
	trigger2, err := handlerConfig.TriggerAble(conf2, handlerConfig.PublisherType)
	s().NoError(err)

	kv1, err := key_value.NewFromInterface(trigger1)
	s().NoError(err)
	kv2, err := key_value.NewFromInterface(trigger2)
	s().NoError(err)

	kvs[0] = kv1
	kvs[1] = kv2

	return req.Ok(key_value.New().Set("handler_configs", kvs))
}

func (test *TestProxySuite) mockedHandlersByRule(req message.RequestInterface) message.ReplyInterface {
	s := test.Require

	fmt.Printf("test.mockedHandlersByRule entered\n")

	ruleKv, err := req.RouteParameters().NestedValue("rule")
	if err != nil {
		return req.Fail("rule parameter is missing")
	}
	var rule service.Rule
	err = ruleKv.Interface(&rule)
	if err != nil {
		return req.Fail(fmt.Sprintf("failed: %v", err))
	}

	categories := []string{"sync_replier", "replier", "pair"}
	for i := range rule.Categories {
		if !slices.Contains(categories, rule.Categories[i]) {
			return req.Fail(fmt.Sprintf("the rule doesn't have %s category in %v list", rule.Categories[i], categories))
		}
	}

	if !slices.Contains(rule.Urls, test.parentUrl) {
		return req.Fail(fmt.Sprintf("the rule doesn't have %s url. given: %v", test.url, rule.Urls))
	}

	kvs := make([]key_value.KeyValue, 0, 3)

	conf1, err := handlerConfig.NewHandler(handlerConfig.SyncReplierType, categories[0])
	s().NoError(err)
	conf2, err := handlerConfig.NewHandler(handlerConfig.ReplierType, categories[1])
	s().NoError(err)
	conf3, err := handlerConfig.NewHandler(handlerConfig.SyncReplierType, categories[2])
	s().NoError(err)

	kv1, err := key_value.NewFromInterface(conf1)
	s().NoError(err)
	kv2, err := key_value.NewFromInterface(conf2)
	s().NoError(err)
	kv3, err := key_value.NewFromInterface(conf3)
	s().NoError(err)

	if len(rule.Categories) == 0 {
		kvs = append(kvs, kv1, kv2, kv3)
	} else {
		if slices.Contains(rule.Categories, categories[0]) {
			kvs = append(kvs, kv1)
		}
		if slices.Contains(rule.Categories, categories[1]) {
			kvs = append(kvs, kv2)
		}
		if slices.Contains(rule.Categories, categories[2]) {
			kvs = append(kvs, kv3)
		}
	}

	return req.Ok(key_value.New().Set("handler_configs", kvs))
}

func (test *TestProxySuite) newMockedServiceManager(managerConfig *clientConfig.Client) (*sync_replier.SyncReplier, *handlerConfig.Handler, error) {
	c := &handlerConfig.Handler{
		Type:           handlerConfig.SyncReplierType,
		Category:       "manager",
		InstanceAmount: 1,
		Id:             managerConfig.Id,
		Port:           managerConfig.Port,
	}

	logger, err := log.New("mocked-service-manager", true)
	if err != nil {
		return nil, nil, err
	}

	syncReplier := sync_replier.New()
	syncReplier.SetConfig(c)
	err = syncReplier.SetLogger(logger)
	if err != nil {
		return nil, nil, err
	}

	err = syncReplier.Route(manager.ProxyChainsByLastId, test.mockedProxyChainsByLastProxy)
	if err != nil {
		return nil, nil, err
	}

	return syncReplier, c, nil
}

// Test_10_NewProxy tests NewProxy
func (test *TestProxySuite) Test_10_NewProxy() {
	s := test.Suite.Require

	_, parentStr, err := ParentConfig(test.parentUrl, test.parentId, uint64(6000))
	s().NoError(err)

	win.Args = append(win.Args,
		arg.NewFlag(flag.IdFlag, test.id),
		arg.NewFlag(flag.UrlFlag, test.url),
		arg.NewFlag(flag.ParentFlag, parentStr),
	)

	proxy, err := NewProxy()
	s().NoError(err)

	// Clean out
	DeleteLastFlags(3)
	s().NoError(proxy.ctx.Close())
	time.Sleep(time.Millisecond * 100)
}

// Test_11_Proxy_SetHandler tests that SetHandler is not invokable in the proxy.
func (test *TestProxySuite) Test_11_Proxy_SetHandler() {
	s := test.Suite.Require

	// Creating a proxy with the valid flags must succeed
	_, parentStr, err := ParentConfig(test.parentUrl, test.parentId, uint64(6000))
	s().NoError(err)

	win.Args = append(win.Args,
		arg.NewFlag(flag.IdFlag, test.id),
		arg.NewFlag(flag.UrlFlag, test.url),
		arg.NewFlag(flag.ParentFlag, parentStr),
	)

	proxy, err := NewProxy()
	s().NoError(err)

	// No handlers were given
	s().Len(proxy.Handlers, 0)

	// Setting handlers won't take any effect
	proxy.SetHandler(test.handlerCategory, test.handler)
	s().Len(proxy.Handlers, 0)

	// Clean out
	DeleteLastFlags(3)
	s().NoError(proxy.ctx.Close())
	time.Sleep(time.Millisecond * 100)
}

// Test_12_Proxy_lintProxyChain checks syncing the proxy chain with a parent.
//
// Todo: test linting a proxy chain from two parents.
// For now, proxy redirects to the one parent only. But in the future it can redirect.
func (test *TestProxySuite) Test_12_Proxy_lintProxyChain() {
	s := test.Require

	parentService := test.parentConfig.Service(test.parentId)
	s().NotNil(parentService)
	parentManager := parentService.Manager
	parentManager.UrlFunc(clientConfig.Url)
	parentKv, err := key_value.NewFromInterface(parentManager)
	s().NoError(err)

	mockedManager, mockedConfig, err := test.newMockedServiceManager(parentManager)
	s().NoError(err)

	// before we start the mocked service, let's add a proxy chain

	localEmpty := &service.Local{}
	// not exists, but we don't care since its upper level and parent won't manage it.
	proxy1 := &service.Proxy{
		Local:    localEmpty,
		Id:       "non_existing_1",
		Url:      "github.com/ahmetson/non-existing",
		Category: "non_existing",
	}
	thisProxy := &service.Proxy{
		Local:    &service.Local{},
		Id:       test.id,
		Url:      test.url,
		Category: "test-proxy",
	}
	serviceRule := service.NewServiceDestination(test.parentUrl)
	proxyChain, err := service.NewProxyChain([]*service.Proxy{proxy1, thisProxy}, serviceRule)
	s().NoError(err)
	s().True(proxyChain.IsValid())
	test.parentProxyChains = []*service.ProxyChain{proxyChain}

	// start the parent manager that will be connected by the proxy
	err = mockedManager.Start()
	s().NoError(err)

	mockedManagerClient, err := manager_client.New(mockedConfig)
	s().NoError(err)

	win.Args = append(win.Args,
		arg.NewFlag(flag.IdFlag, test.id),
		arg.NewFlag(flag.UrlFlag, test.url),
		arg.NewFlag(flag.ParentFlag, parentKv.String()),
	)

	// let's create our proxy
	proxy, err := NewProxy()
	s().NoError(err)
	DeleteLastFlags(3)

	parentProxyChains, err := proxy.ParentManager.ProxyChainsByLastProxy(proxy.id)
	s().NoError(err)
	s().Len(parentProxyChains, 1)

	// linting a proxy chain requires dep manager and proxy handler in the context
	proxy.ctx.SetService(test.id, test.url)
	err = proxy.ctx.StartDepManager()
	s().NoError(err)
	err = proxy.ctx.StartProxyHandler()
	s().NoError(err)

	// before linting with parent,
	// the Proxy must not have any proxies
	proxyClient := proxy.ctx.ProxyClient()
	proxyChains, err := proxyClient.ProxyChains()
	s().NoError(err)
	s().Len(proxyChains, 0)
	s().Nil(proxy.rule)
	dest, err := proxy.destination()
	s().Nil(dest)
	s().Error(err)

	// Linting
	err = proxy.lintProxyChain()
	s().NoError(err)

	proxyChains, err = proxyClient.ProxyChains()
	s().NoError(err)
	s().Len(proxyChains, 1)
	s().Nil(proxy.rule)
	dest, err = proxy.destination()
	s().NotNil(dest)
	s().NoError(err)

	// Clean-out.
	// Test as the proxy is the first
	err = proxy.ctx.Close()
	s().NoError(err)

	// Wait a bit for close of the threads
	time.Sleep(time.Millisecond * 100)

	win.Args = append(win.Args,
		arg.NewFlag(flag.IdFlag, test.id),
		arg.NewFlag(flag.UrlFlag, test.url),
		arg.NewFlag(flag.ParentFlag, parentKv.String()),
	)

	// let's create our proxy
	proxy, err = NewProxy()
	s().NoError(err)
	DeleteLastFlags(3)

	proxy.ctx.SetService(test.id, test.url)
	err = proxy.ctx.StartDepManager()
	s().NoError(err)
	err = proxy.ctx.StartProxyHandler()
	s().NoError(err)

	// Parent must have a proxy with one data
	proxyChain, err = service.NewProxyChain([]*service.Proxy{thisProxy}, serviceRule)
	s().NoError(err)
	s().True(proxyChain.IsValid())
	test.parentProxyChains = []*service.ProxyChain{proxyChain}

	// Lint as this proxy is the first
	proxyClient = proxy.ctx.ProxyClient()
	proxyChains, err = proxyClient.ProxyChains()
	s().NoError(err)
	s().Len(proxyChains, 0)
	s().Nil(proxy.rule)
	dest, err = proxy.destination()
	s().Nil(dest)
	s().Error(err)

	// Linting
	err = proxy.lintProxyChain()
	s().NoError(err)

	proxyChains, err = proxyClient.ProxyChains()
	s().NoError(err)
	s().Len(proxyChains, 0)
	s().NotNil(proxy.rule)
	dest, err = proxy.destination()
	s().NotNil(dest)
	s().NoError(err)

	err = mockedManagerClient.Close()
	s().NoError(err)

	err = proxy.ctx.Close()
	s().NoError(err)

	// Wait a bit for close of the threads
	time.Sleep(time.Millisecond * 100)
}

// Test_13_Proxy_lintHandlers makes sure that proxy gets the handlers from the parent.
//
// Todo: Testing a various types of the handlers by different rules.
// Todo: Testing the triggers are not supported
// Todo: change the proxy's rule directly.
// Todo: design a trigger-able proxy accepting.
// Todo: design a trigger-able in the service.
//
// Todo: test a service with the trigger-able handler.
func (test *TestProxySuite) Test_13_Proxy_lintHandlers() {
	s := test.Require

	// Deriving the parent's manager configuration
	// from _test_services/proxy_parent/backend/bin/app.yml
	parentService := test.parentConfig.Service(test.parentId)
	s().NotNil(parentService)
	parentManager := parentService.Manager
	parentManager.UrlFunc(clientConfig.Url)
	parentKv, err := key_value.NewFromInterface(parentManager)
	s().NoError(err)

	mockedManager, mockedConfig, err := test.newMockedServiceManager(parentManager)
	s().NoError(err)

	// Let's over-write the routing for the handlers.

	err = mockedManager.Route(manager.HandlersByRule, test.mockedHandlersByRuleEmpty)
	s().NoError(err)

	err = mockedManager.Start()
	s().NoError(err)

	// wait a bit for initialization
	time.Sleep(time.Millisecond * 100)

	win.Args = append(win.Args,
		arg.NewFlag(flag.IdFlag, test.id),
		arg.NewFlag(flag.UrlFlag, test.url),
		arg.NewFlag(flag.ParentFlag, parentKv.String()),
	)

	// let's create our proxy
	proxy, err := NewProxy()
	s().NoError(err)
	DeleteLastFlags(3)

	//// init the config
	//proxy.ctx.SetService(test.id, test.url)

	// No handlers
	s().Len(proxy.Handlers, 0)

	rule := service.NewServiceDestination(parentService.Url)
	proxy.rule = rule // available in the proxy as proxy.destination()

	// 1. fail, handlers are not set in the parent.
	err = proxy.lintHandlers()
	s().Error(err)

	// 2. succeed partially, handler returns trigger-able config which is converted to base interface.
	// Over-writing the route while the service is running is not possible.

	// restarting to set new route handle function
	mockedManagerClient, err := manager_client.New(mockedConfig)
	s().NoError(err)
	err = mockedManagerClient.Close()
	s().NoError(err)
	time.Sleep(time.Millisecond * 100) // Wait a bit for parent closing

	mockedManager, mockedConfig, err = test.newMockedServiceManager(parentManager)
	s().NoError(err)
	err = mockedManager.Route(manager.HandlersByRule, test.mockedHandlersByRuleTriggers)
	s().NoError(err)
	err = mockedManager.Start()
	s().NoError(err)
	err = proxy.ctx.Close()
	s().NoError(err)

	time.Sleep(time.Millisecond * 100) // Wait a bit for parent initiation

	win.Args = append(win.Args,
		arg.NewFlag(flag.IdFlag, test.id),
		arg.NewFlag(flag.UrlFlag, test.url),
		arg.NewFlag(flag.ParentFlag, parentKv.String()),
	)
	proxy, err = NewProxy() // restarting so that parent manager is a new client
	s().NoError(err)
	DeleteLastFlags(3)
	proxy.rule = rule // available in the proxy as proxy.destination()

	time.Sleep(time.Millisecond * 100) // Wait a bit for parent initiation

	err = proxy.lintHandlers()
	s().NoError(err)

	s().Len(proxy.Handlers, 2)

	//
	// 3. parent has 3 services.
	//

	// 3.1 fail!
	// trying invalid url
	// clear out the proxy services first to test again
	// restarting to set new route handle function
	mockedManagerClient, err = manager_client.New(mockedConfig)
	s().NoError(err)
	err = mockedManagerClient.Close()
	s().NoError(err)
	time.Sleep(time.Millisecond * 100) // Wait a bit for parent closing

	mockedManager, mockedConfig, err = test.newMockedServiceManager(parentManager)
	s().NoError(err)
	err = mockedManager.Route(manager.HandlersByRule, test.mockedHandlersByRule)
	s().NoError(err)
	err = mockedManager.Start()
	s().NoError(err)
	err = proxy.ctx.Close()
	s().NoError(err)

	time.Sleep(time.Millisecond * 100) // Wait a bit for parent initiation

	win.Args = append(win.Args,
		arg.NewFlag(flag.IdFlag, test.id),
		arg.NewFlag(flag.UrlFlag, test.url),
		arg.NewFlag(flag.ParentFlag, parentKv.String()),
	)
	proxy, err = NewProxy() // restarting so that parent manager is a new client
	s().NoError(err)
	DeleteLastFlags(3)

	time.Sleep(time.Millisecond * 100) // Wait a bit for parent initiation

	rule = service.NewServiceDestination([]string{"no_url_1", "no_url_2"})
	proxy.rule = rule
	fmt.Printf("rule is nil? %v, the proxy rule is nil? %v of %v\n", rule, proxy.rule == nil, proxy)
	err = proxy.lintHandlers()
	s().Error(err)

	// 3.2 success
	// normal fetch by service url
	proxy.rule = service.NewServiceDestination([]string{parentService.Url, "no_url_1"})
	s().Len(proxy.Handlers, 0)

	err = proxy.lintHandlers()
	s().NoError(err)
	s().Len(proxy.Handlers, 3)

	// 3.3 fail
	// by handler rule, no category exists
	proxy.Handlers = key_value.New() // clean out

	proxy.rule = service.NewHandlerDestination(
		parentService.Url, "no_category")
	s().Len(proxy.Handlers, 0)
	err = proxy.lintHandlers()
	s().Error(err)

	// 3.4 success
	// by handler rule, a one category
	proxy.Handlers = key_value.New() // clean out
	fmt.Printf("3.4 before linting: %d\n", len(proxy.Handlers))
	proxy.rule = service.NewHandlerDestination(parentService.Url, "sync_replier")
	err = proxy.lintHandlers()
	s().NoError(err)
	fmt.Printf("3.4 after linting: %d\n", len(proxy.Handlers))
	s().Len(proxy.Handlers, 1)

	// fetch all handlers
	proxy.Handlers = key_value.New() // clean out
	proxy.rule = service.NewHandlerDestination(parentService.Url,
		[]string{"sync_replier", "replier", "pair"})
	err = proxy.lintHandlers()
	s().NoError(err)
	s().Len(proxy.Handlers, 3)

	// 3.5 success
	// by route category
	//
	proxy.Handlers = key_value.New() // clean out
	proxy.rule = service.NewDestination(parentService.Url,
		[]string{"sync_replier", "replier", "pair"}, "command")
	err = proxy.lintHandlers()
	s().Len(proxy.Handlers, 3)

	// Clean out
	mockedManagerClient, err = manager_client.New(mockedConfig)
	s().NoError(err)

	err = mockedManagerClient.Close()
	s().NoError(err)

	err = proxy.ctx.Close()
	s().NoError(err)

	// Wait a bit for close of the threads
	time.Sleep(time.Millisecond * 100)
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
