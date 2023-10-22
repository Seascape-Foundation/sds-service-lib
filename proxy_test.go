package service

import (
	"fmt"
	"github.com/ahmetson/client-lib"
	clientConfig "github.com/ahmetson/client-lib/config"
	"github.com/ahmetson/config-lib/app"
	"github.com/ahmetson/config-lib/service"
	"github.com/ahmetson/datatype-lib/data_type/key_value"
	"github.com/ahmetson/datatype-lib/message"
	"github.com/ahmetson/handler-lib/base"
	handlerConfig "github.com/ahmetson/handler-lib/config"
	"github.com/ahmetson/handler-lib/manager_client"
	"github.com/ahmetson/handler-lib/replier"
	"github.com/ahmetson/handler-lib/route"
	"github.com/ahmetson/handler-lib/sync_replier"
	"github.com/ahmetson/log-lib"
	"github.com/ahmetson/os-lib/arg"
	"github.com/ahmetson/os-lib/path"
	"github.com/ahmetson/service-lib/flag"
	"github.com/ahmetson/service-lib/manager"
	"github.com/pebbe/zmq4"
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

	backend      *zmq4.Socket
	workerResult string
	replied      string
	request      string

	categories     []string
	units          []*service.Unit
	handlerConfigs []*handlerConfig.Handler
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
	test.units = []*service.Unit{}
	test.handlerConfigs = []*handlerConfig.Handler{}
}

func (test *TestProxySuite) mockedProxyChainsByLastProxy(req message.RequestInterface) message.ReplyInterface {
	id, err := req.RouteParameters().StringValue("id")
	if err != nil {
		return req.Fail("id parameter is missing")
	}
	proxyChains := make([]key_value.KeyValue, 0, 1)

	if test.id != id || len(test.parentProxyChains) == 0 {
		return req.Ok(key_value.New().Set("proxy_chains", proxyChains))
	}

	for i := range test.parentProxyChains {
		kv, err := key_value.NewFromInterface(test.parentProxyChains[i])
		if err != nil {
			return req.Fail(fmt.Sprintf("test.parentProxyChains[%d]: %v", i, err))
		}
		proxyChains = append(proxyChains, kv)
	}

	return req.Ok(key_value.New().Set("proxy_chains", proxyChains))
}

func (test *TestProxySuite) mockedHandlersByRuleEmpty(req message.RequestInterface) message.ReplyInterface {
	kvs := make([]key_value.KeyValue, 0)

	return req.Ok(key_value.New().Set("handler_configs", kvs))
}

func (test *TestProxySuite) mockedHandlersByRuleTriggers(req message.RequestInterface) message.ReplyInterface {
	s := test.Require

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

func (test *TestProxySuite) newHandlers() {
	s := test.Require

	test.categories = []string{"sync_replier", "replier", "pair"}

	conf1, err := handlerConfig.NewHandler(handlerConfig.SyncReplierType, test.categories[0])
	s().NoError(err)
	conf2, err := handlerConfig.NewHandler(handlerConfig.ReplierType, test.categories[1])
	s().NoError(err)
	conf3, err := handlerConfig.NewHandler(handlerConfig.SyncReplierType, test.categories[2])
	s().NoError(err)
	test.handlerConfigs = []*handlerConfig.Handler{conf1, conf2, conf3}
}

func (test *TestProxySuite) mockedHandlersByRule(req message.RequestInterface) message.ReplyInterface {
	s := test.Require

	if len(test.handlerConfigs) == 0 {
		test.newHandlers()
	}

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

	kv1, err := key_value.NewFromInterface(test.handlerConfigs[0])
	s().NoError(err)
	kv2, err := key_value.NewFromInterface(test.handlerConfigs[1])
	s().NoError(err)
	kv3, err := key_value.NewFromInterface(test.handlerConfigs[2])
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

// Test lintUnits when units are empty
func (test *TestProxySuite) mockedEmptyUnits(req message.RequestInterface) message.ReplyInterface {
	kvs := make([]key_value.KeyValue, 0)

	return req.Ok(key_value.New().Set("units", kvs))
}

func (test *TestProxySuite) newUnits() {
	if len(test.handlerConfigs) == 0 {
		test.newHandlers()
	}
	conf1 := service.Unit{
		ServiceId: test.parentId,
		HandlerId: test.handlerConfigs[0].Id,
		Command:   "hello",
	}
	conf2 := service.Unit{
		ServiceId: test.parentId,
		HandlerId: test.handlerConfigs[1].Id,
		Command:   "world",
	}
	conf3 := service.Unit{
		ServiceId: test.parentId,
		HandlerId: test.handlerConfigs[2].Id,
		Command:   "hello",
	}
	test.units = []*service.Unit{&conf1, &conf2, &conf3}
}

// Has the four rules:
//
//	Service Rule for the test.parentUrl
//	Handler Rule for sync_replier
//	Handler Rule for replier
//	Route Rule for the test.parentUrl all routes.
func (test *TestProxySuite) mockedUnits(req message.RequestInterface) message.ReplyInterface {
	s := test.Require

	if len(test.units) == 0 {
		test.newUnits()
	}

	raw, err := req.RouteParameters().NestedValue("rule")
	if err != nil {
		return req.Fail(fmt.Sprintf("req.RouteParameters().NestedValue('proxy_chain'): %v", err))
	}

	var rule service.Rule
	err = raw.Interface(&rule)
	if err != nil {
		return req.Fail(fmt.Sprintf("interface conversion failed for %v", rule))
	}

	kvs := make([]key_value.KeyValue, 0, 3)

	kv1, err := key_value.NewFromInterface(test.units[0])
	s().NoError(err)
	kv2, err := key_value.NewFromInterface(test.units[1])
	s().NoError(err)
	kv3, err := key_value.NewFromInterface(test.units[2])
	s().NoError(err)

	if rule.IsService() && rule.Urls[0] == test.parentUrl {
		kvs = append(kvs, kv1, kv2, kv3)
	} else if rule.IsHandler() {
		if slices.Contains(rule.Categories, "sync_replier") {
			kvs = append(kvs, kv1, kv2)
		}
		if slices.Contains(rule.Categories, "replier") {
			kvs = append(kvs, kv3)
		}
	} else if rule.IsRoute() {
		kvs = append(kvs, kv1, kv2, kv3)
	}

	return req.Ok(key_value.New().Set("units", kvs))
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

func (test *TestProxySuite) runBackend(amount int, url string, zmqType zmq4.Type) {
	require := test.Require

	var err error
	test.backend, err = zmq4.NewSocket(zmqType)
	require().NoError(err)

	err = test.backend.Bind(url)
	require().NoError(err)

	for i := 1; i <= amount; i++ {
		msg, err := test.backend.RecvMessage(0)
		if err == zmq4.ErrorSocketClosed {
			return
		}
		require().NoError(err)

		if zmqType == zmq4.PULL {
			test.workerResult = fmt.Sprintf("%d", i)
			continue
		}
		req, err := message.NewReq(msg)
		require().NoError(err)
		reply := req.Ok(req.RouteParameters().Set("backend", true))
		replyStr, err := reply.ZmqEnvelope()
		require().NoError(err)
		_, err = test.backend.SendMessage(replyStr)
		require().NoError(err)
	}

	err = test.backend.Close()
	require().NoError(err)

	test.backend = nil
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
	err = mockedManager.Route(manager.ProxyConfigSet, test.mockedEmptyUnits)
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
	err = mockedManager.Route(manager.ProxyConfigSet, test.mockedEmptyUnits)
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
	err = mockedManager.Route(manager.ProxyConfigSet, test.mockedEmptyUnits)
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
	proxy.rule = service.NewHandlerDestination(parentService.Url, "sync_replier")
	err = proxy.lintHandlers()
	s().NoError(err)
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

// Test_14_Proxy_setProxyUnits test over-writing the proxy units.
// Tests fetching units from the parent.
func (test *TestProxySuite) Test_14_Proxy_setProxyUnits() {
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

	err = mockedManager.Route(manager.Units, test.mockedUnits)
	s().NoError(err)
	err = mockedManager.Route(manager.ProxyConfigSet, test.mockedEmptyUnits)
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
	proxy.ctx.SetService(test.id, test.url)
	err = proxy.ctx.StartDepManager()
	s().NoError(err)
	err = proxy.ctx.StartProxyHandler()
	s().NoError(err)

	// wait a bit for initialization
	time.Sleep(time.Millisecond * 100)

	// No handlers
	s().Len(proxy.Handlers, 0)

	rule := service.NewServiceDestination(parentService.Url)
	proxy.rule = rule // available in the proxy as proxy.destination()

	// 1. fail, handlers are not set in the parent.
	err = proxy.setProxyUnits()
	s().NoError(err)

	// Clean out
	mockedManagerClient, err := manager_client.New(mockedConfig)
	s().NoError(err)

	err = mockedManagerClient.Close()
	s().NoError(err)

	err = proxy.ctx.Close()
	s().NoError(err)

	test.units = []*service.Unit{}

	// Wait a bit for closing the threads
	time.Sleep(time.Millisecond * 100)
}

// Test_15_Proxy_Start tests Proxy.Start method
func (test *TestProxySuite) Test_15_Proxy_Start() {
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

	// not exists, but we don't care since its upper level and parent won't manage it.
	thisProxy := &service.Proxy{
		Local:    &service.Local{},
		Id:       test.id,
		Url:      test.url,
		Category: "test-proxy",
	}
	serviceRule := service.NewServiceDestination(test.parentUrl)
	proxyChain, err := service.NewProxyChain([]*service.Proxy{thisProxy}, serviceRule)
	s().NoError(err)
	s().True(proxyChain.IsValid())
	test.parentProxyChains = []*service.ProxyChain{proxyChain}

	err = mockedManager.Route(manager.HandlersByRule, test.mockedHandlersByRule)
	s().NoError(err)
	err = mockedManager.Route(manager.Units, test.mockedUnits)
	s().NoError(err)
	err = mockedManager.Route(manager.ProxyConfigSet, test.mockedEmptyUnits)
	s().NoError(err)

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

	// Starting must initialize the handlers too
	s().Zero(len(proxy.Handlers))
	_, err = proxy.Start()
	s().NoError(err)

	// Wait a bit for initialization...
	time.Sleep(time.Millisecond * 100)
	s().NotZero(len(proxy.Handlers))

	// Clean out
	err = mockedManagerClient.Close()
	s().NoError(err)

	err = proxy.manager.Close()
	s().NoError(err)

	test.units = []*service.Unit{}
	test.handlerConfigs = []*handlerConfig.Handler{}

	// Wait a bit for close of the threads
	time.Sleep(time.Millisecond * 100)
}

// Test_16_HandleFunctions tests that handle functions in the proxy works
//
// Todo test with the submits if the handler is the puller
func (test *TestProxySuite) Test_16_Proxy_routeWrapper() {
	s := test.Require

	helloWrapper := &HandlerWrapper{}
	helloId := "hello"

	parentService := test.parentConfig.Service(test.parentId)
	s().NotNil(parentService)
	parentManager := parentService.Manager
	parentManager.UrlFunc(clientConfig.Url)
	parentKv, err := key_value.NewFromInterface(parentManager)
	s().NoError(err)

	//mockedManager, mockedConfig, err := test.newMockedServiceManager(parentManager)
	//s().NoError(err)

	win.Args = append(win.Args,
		arg.NewFlag(flag.IdFlag, test.id),
		arg.NewFlag(flag.UrlFlag, test.url),
		arg.NewFlag(flag.ParentFlag, parentKv.String()),
	)

	// let's create our proxy
	proxy, err := NewProxy()
	s().NoError(err)
	DeleteLastFlags(3)

	// fail as proxy.handleWrappers == empty
	reply := proxy.routeWrapper(helloId, &message.Request{})
	s().False(reply.IsOK())

	// fail if proxy.handleWrappers[non_existing_id]
	proxy.handlerWrappers[helloId] = helloWrapper
	reply = proxy.routeWrapper("non_existing", &message.Request{})
	s().False(reply.IsOK())

	// set the proxy type as submitting (create a handler of trigger type)
	// make sure that reply is ok, but has no parameter
	workerConf := handlerConfig.NewInternalHandler(handlerConfig.WorkerType, "sample_worker")
	workerZmqType := handlerConfig.SocketType(workerConf.Type)
	pusherConf := clientConfig.New("", workerConf.Id, workerConf.Port, workerZmqType)
	pusherConf.UrlFunc(clientConfig.Url)
	pusher, err := client.New(pusherConf)
	s().NoError(err)
	workerUrl := handlerConfig.ExternalUrl(pusherConf.Id, pusherConf.Port)

	// worker closes after workAmount.
	// 1. to test submit purely
	// 2. to test onReply have no effect
	// test onRequest with intentional error is not submitting, so skip it
	// 3. to test onRequest without any error
	workAmount := 3
	go test.runBackend(workAmount, workerUrl, workerZmqType)
	time.Sleep(time.Millisecond * 100) // wait a bit for backend initialization

	helloWrapper.destConfig = workerConf
	helloWrapper.destClient = pusher
	proxy.handlerWrappers[helloId] = helloWrapper

	s().Empty(test.workerResult)
	reply = proxy.routeWrapper(helloId, &message.Request{Command: "cmd", Parameters: key_value.New()})
	s().True(reply.IsOK())

	// Wait a bit for the effect
	time.Sleep(time.Millisecond * 100)
	s().NotEmpty(test.workerResult)
	test.workerResult = ""

	// onReply call is tracked by setting a string on test.replied
	// setting onReply won't take any effect
	proxy.onReply = func(handlerId string, req message.RequestInterface, rep message.ReplyInterface) (message.ReplyInterface, error) {
		test.replied = "handler_id_1"
		return rep, nil
	}
	s().Empty(test.replied)

	reply = proxy.routeWrapper(helloId, &message.Request{Command: "cmd", Parameters: key_value.New()})
	s().True(reply.IsOK())
	time.Sleep(time.Millisecond * 100) // wait a bit for sending

	s().Empty(test.replied)
	s().NotEmpty(test.workerResult)
	test.workerResult = ""

	// onRequest that fails must not submit the message
	proxy.onRequest = func(handlerId string, req message.RequestInterface) (message.RequestInterface, error) {
		test.request = "on_request_failed"
		return nil, fmt.Errorf("intentionally failed")
	}
	s().Empty(test.request)

	reply = proxy.routeWrapper(helloId, &message.Request{Command: "cmd", Parameters: key_value.New()})
	s().False(reply.IsOK()) // we get it immediately

	s().Empty(test.replied)
	s().Empty(test.workerResult)
	s().Equal("on_request_failed", test.request)
	test.request = ""

	// request must over-write the next
	proxy.onRequest = func(handlerId string, req message.RequestInterface) (message.RequestInterface, error) {
		test.request = "on_request_succeed"
		return req, nil
	}
	s().Empty(test.request)

	reply = proxy.routeWrapper(helloId, &message.Request{Command: "cmd", Parameters: key_value.New()})
	s().True(reply.IsOK())

	// test a bit for the submission to get effect by the handler
	time.Sleep(time.Millisecond * 100)

	s().Empty(test.replied)
	s().NotEmpty(test.workerResult)
	s().Equal("on_request_succeed", test.request)
	test.request = ""
	test.workerResult = ""

	// close the submission and use a sync replier handler as a destination
	// Clean out for submitting
	if test.backend != nil {
		s().NoError(test.backend.Close())
	}

	err = helloWrapper.destClient.Close()
	s().NoError(err)

	// change the helloWrapper.destConf to a request
	// create a sync_replier and request dest client
	replierConf := handlerConfig.NewInternalHandler(handlerConfig.SyncReplierType, "sample_sync_replier")
	replierZmqType := handlerConfig.SocketType(replierConf.Type)
	reqConf := clientConfig.New("", replierConf.Id, replierConf.Port, replierZmqType)
	reqConf.UrlFunc(clientConfig.Url)
	reqClient, err := client.New(reqConf)
	s().NoError(err)
	replierUrl := handlerConfig.ExternalUrl(reqConf.Id, reqConf.Port)

	// 1. Normal request without onReply and onRequest
	// - onRequest fails it's not counted
	// 2. Normal onRequest
	// 3. onReply fails
	// 4. onReply succeeds
	replyAmount := 4
	go test.runBackend(replyAmount, replierUrl, replierZmqType)
	time.Sleep(time.Millisecond * 100) // wait a bit for backend initialization

	helloWrapper.destConfig = replierConf
	helloWrapper.destClient = reqClient
	proxy.handlerWrappers[helloId] = helloWrapper

	s().Empty(test.workerResult)
	proxy.onRequest = nil
	proxy.onReply = nil
	reply = proxy.routeWrapper(helloId, &message.Request{Command: "cmd", Parameters: key_value.New()})
	s().True(reply.IsOK())
	response, err := reply.ReplyParameters().BoolValue("backend")
	s().NoError(err)
	s().True(response)

	// Request must fail intentionally
	proxy.onRequest = func(handlerId string, req message.RequestInterface) (message.RequestInterface, error) {
		test.request = "on_request_failed"
		return nil, fmt.Errorf("failed intentionally")
	}
	s().Empty(test.request)
	reply = proxy.routeWrapper(helloId, &message.Request{Command: "cmd", Parameters: key_value.New()})
	s().False(reply.IsOK())
	s().Equal("on_request_failed", test.request)
	s().Len(reply.ReplyParameters(), 0)
	test.request = ""

	// Request succeeds
	proxy.onRequest = func(handlerId string, req message.RequestInterface) (message.RequestInterface, error) {
		test.request = "on_request_succeed"
		req.RouteParameters().Set("on_request", true)
		return req, nil
	}
	s().Empty(test.request)
	reply = proxy.routeWrapper(helloId, &message.Request{Command: "cmd", Parameters: key_value.New()})
	s().True(reply.IsOK())
	s().Equal("on_request_succeed", test.request)
	test.request = ""
	// the destination added the parameters
	response, err = reply.ReplyParameters().BoolValue("backend")
	s().NoError(err)
	s().True(response)
	// the onRequest added the parameters
	response, err = reply.ReplyParameters().BoolValue("on_request")
	s().NoError(err)
	s().True(response)

	// On reply fails intentionally
	proxy.onReply = func(handlerId string, req message.RequestInterface, rep message.ReplyInterface) (message.ReplyInterface, error) {
		test.replied = "on_reply_failed"
		return nil, fmt.Errorf("failed intentionally")
	}
	s().Empty(test.replied)
	s().Empty(test.request)
	reply = proxy.routeWrapper(helloId, &message.Request{Command: "cmd", Parameters: key_value.New()})
	s().False(reply.IsOK())
	s().Equal("on_request_succeed", test.request)
	s().Equal("on_reply_failed", test.replied)
	test.request = ""
	test.replied = ""

	// onReply succeeds intentionally
	proxy.onReply = func(handlerId string, req message.RequestInterface, rep message.ReplyInterface) (message.ReplyInterface, error) {
		test.replied = "on_reply_succeed"
		rep.ReplyParameters().Set("on_reply", true)
		return rep, nil
	}
	s().Empty(test.replied)
	s().Empty(test.request)
	reply = proxy.routeWrapper(helloId, &message.Request{Command: "cmd", Parameters: key_value.New()})
	s().True(reply.IsOK())
	s().Equal("on_request_succeed", test.request)
	s().Equal("on_reply_succeed", test.replied)
	test.request = ""
	test.replied = ""
	// the destination added the parameters
	response, err = reply.ReplyParameters().BoolValue("backend")
	s().NoError(err)
	s().True(response)
	// the onRequest added the parameters
	response, err = reply.ReplyParameters().BoolValue("on_request")
	s().NoError(err)
	s().True(response)
	// the onReply added the parameters
	response, err = reply.ReplyParameters().BoolValue("on_reply")
	s().NoError(err)
	s().True(response)

	// Clean out for request
	if test.backend != nil {
		s().NoError(test.backend.Close())
	}

	err = helloWrapper.destClient.Close()
	s().NoError(err)

	err = proxy.ctx.Close()
	s().NoError(err)

	// Wait a bit for close of the threads
	time.Sleep(time.Millisecond * 100)
}

// Test_17_Proxy_routeHandlers test routeHandlers by applying handler calls for each unit
func (test *TestProxySuite) Test_17_Proxy_routeHandlers() {
	s := test.Require

	test.newHandlers()
	test.newUnits()

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

	// start the handlers for the parent
	parent0Handler := sync_replier.New()
	parent0Handler.SetConfig(test.handlerConfigs[0])
	err = parent0Handler.SetLogger(test.logger)
	s().NoError(err)
	err = parent0Handler.Route(test.units[0].Command, func(req message.RequestInterface) message.ReplyInterface {
		params := req.RouteParameters().Set("parent_command", req.CommandName()).Set("handler_index", 0)
		return req.Ok(params)
	})
	s().NoError(err)

	parent1Handler := replier.New()
	parent1Handler.SetConfig(test.handlerConfigs[1])
	err = parent1Handler.SetLogger(test.logger)
	s().NoError(err)
	err = parent1Handler.Route(test.units[1].Command, func(req message.RequestInterface) message.ReplyInterface {
		params := req.RouteParameters().Set("parent_command", req.CommandName()).Set("handler_index", 1)
		return req.Ok(params)
	})
	s().NoError(err)

	parent2Handler := sync_replier.New()
	parent2Handler.SetConfig(test.handlerConfigs[2])
	err = parent2Handler.SetLogger(test.logger)
	s().NoError(err)
	err = parent2Handler.Route(test.units[2].Command, func(req message.RequestInterface) message.ReplyInterface {
		params := req.RouteParameters().Set("parent_command", req.CommandName()).Set("handler_index", 2)
		return req.Ok(params)
	})
	s().NoError(err)

	err = parent0Handler.Start()
	s().NoError(err)
	err = parent1Handler.Start()
	s().NoError(err)
	err = parent2Handler.Start()
	s().NoError(err)

	// Wait a bit for initialization
	time.Sleep(time.Millisecond * 100)

	// Let's over-write the routing for the handlers.

	err = mockedManager.Route(manager.Units, test.mockedUnits)
	s().NoError(err)
	err = mockedManager.Route(manager.ProxyConfigSet, test.mockedEmptyUnits)
	s().NoError(err)
	err = mockedManager.Route(manager.HandlersByRule, test.mockedHandlersByRule)
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
	proxy.ctx.SetService(test.id, test.url)
	err = proxy.ctx.StartDepManager()
	s().NoError(err)
	err = proxy.ctx.StartProxyHandler()
	s().NoError(err)

	// wait a bit for initialization
	time.Sleep(time.Millisecond * 100)

	// No handlers
	s().Len(proxy.Handlers, 0)

	rule := service.NewServiceDestination(parentService.Url)
	proxy.rule = rule // available in the proxy as proxy.destination()

	err = proxy.SetReplyHandler(func(handlerId string, req message.RequestInterface, rep message.ReplyInterface) (message.ReplyInterface, error) {
		for key, value := range req.RouteParameters() {
			rep.ReplyParameters().Set(key, value)
		}
		rep.ReplyParameters().Set("reply_handler_id", handlerId).Set("reply_command", req.CommandName())
		return rep, nil
	})
	s().NoError(err)
	err = proxy.SetRequestHandler(func(handlerId string, req message.RequestInterface) (message.RequestInterface, error) {
		req.RouteParameters().Set("request_handler_id", handlerId).Set("request_command", req.CommandName())
		return req, nil
	})
	s().NoError(err)

	// prepare the parent data
	s().Empty(proxy.Handlers)
	s().Empty(proxy.handlerWrappers)

	err = proxy.lintProxyChain()
	s().NoError(err)
	err = proxy.lintHandlers()
	s().NoError(err)
	err = proxy.setProxyUnits() // calls routeHandlers
	s().NoError(err)

	time.Sleep(time.Millisecond * 100)

	s().NotEmpty(proxy.Handlers)
	s().NotEmpty(proxy.handlerWrappers)
	raw, ok := proxy.Handlers[test.id+test.categories[0]]
	s().True(ok)
	handler := raw.(base.Interface)
	s().NotEmpty(handler.RouteCommands())

	// Generate the configuration
	err = proxy.setConfig()
	s().NoError(err)
	err = proxy.newManager()
	s().NoError(err)

	// Wait a bit for proxy preparation
	time.Sleep(time.Millisecond * 100)

	// Proxy handlers must run too
	err = proxy.startHandlers()
	s().NoError(err)

	raw, ok = proxy.Handlers[test.id+test.categories[0]]
	s().True(ok)
	handler = raw.(base.Interface)
	s().NotEmpty(handler.RouteCommands())
	proxyHandlerConf1 := handler.Config()

	// Wait a bit for proxy preparation
	clientConf1 := clientConfig.New(test.parentUrl, proxyHandlerConf1.Id, proxyHandlerConf1.Port, handlerConfig.SocketType(proxyHandlerConf1.Type))
	clientConf1.UrlFunc(clientConfig.Url)
	client1, err := client.New(clientConf1)
	s().NoError(err)

	raw, ok = proxy.Handlers[test.id+test.categories[1]]
	s().True(ok)
	handler = raw.(base.Interface)
	s().NotEmpty(handler.RouteCommands())
	proxyHandlerConf2 := handler.Config()
	clientConf2 := clientConfig.New(test.parentUrl, proxyHandlerConf2.Id, proxyHandlerConf2.Port, handlerConfig.SocketType(proxyHandlerConf2.Type))
	clientConf2.UrlFunc(clientConfig.Url)
	client2, err := client.New(clientConf2)
	//req to router doesn't work, test it
	s().NoError(err)

	raw, ok = proxy.Handlers[test.id+test.categories[2]]
	s().True(ok)
	handler = raw.(base.Interface)
	s().NotEmpty(handler.RouteCommands())
	proxyHandlerConf3 := handler.Config()
	clientConf3 := clientConfig.New(test.parentUrl, proxyHandlerConf3.Id, proxyHandlerConf3.Port, handlerConfig.SocketType(proxyHandlerConf3.Type))
	clientConf3.UrlFunc(clientConfig.Url)
	client3, err := client.New(clientConf3)
	s().NoError(err)

	// Sending a message that is not registered
	req := message.Request{
		Command:    "non_exist",
		Parameters: key_value.New(),
	}
	reply, err := client1.Request(&req)
	s().NoError(err)
	s().False(reply.IsOK())
	reply, err = client2.Request(&req)
	s().NoError(err)
	s().False(reply.IsOK())
	reply, err = client3.Request(&req)
	s().NoError(err)
	s().False(reply.IsOK())

	// Sending a message to the handler first through the proxy
	i := 0
	req = message.Request{
		Command:    test.units[i].Command,
		Parameters: key_value.New(),
	}
	reply, err = client1.Request(&req)
	s().NoError(err)
	s().True(reply.IsOK())
	parentCmd, err := reply.ReplyParameters().StringValue("parent_command")
	s().NoError(err)
	s().Equal(test.units[i].Command, parentCmd)
	onRequestCmd, err := reply.ReplyParameters().StringValue("request_command")
	s().NoError(err)
	s().Equal(test.units[i].Command, onRequestCmd)
	onReplyCmd, err := reply.ReplyParameters().StringValue("reply_command")
	s().NoError(err)
	s().Equal(test.units[i].Command, onReplyCmd)

	// Sending a message to the second handler through the proxy
	i++
	req = message.Request{
		Command:    test.units[i].Command,
		Parameters: key_value.New(),
	}
	reply, err = client2.Request(&req)
	s().NoError(err)
	s().True(reply.IsOK())

	parentCmd, err = reply.ReplyParameters().StringValue("parent_command")
	s().NoError(err)
	s().Equal(test.units[i].Command, parentCmd)
	onRequestCmd, err = reply.ReplyParameters().StringValue("request_command")
	s().NoError(err)
	s().Equal(test.units[i].Command, onRequestCmd)
	onReplyCmd, err = reply.ReplyParameters().StringValue("reply_command")
	s().NoError(err)
	s().Equal(test.units[i].Command, onReplyCmd)

	// Sending a message to the third handler through the proxy
	i++
	req = message.Request{
		Command:    test.units[i].Command,
		Parameters: key_value.New(),
	}
	reply, err = client3.Request(&req)
	s().NoError(err)
	s().True(reply.IsOK())

	parentCmd, err = reply.ReplyParameters().StringValue("parent_command")
	s().NoError(err)
	s().Equal(test.units[i].Command, parentCmd)
	onRequestCmd, err = reply.ReplyParameters().StringValue("request_command")
	s().NoError(err)
	s().Equal(test.units[i].Command, onRequestCmd)
	onReplyCmd, err = reply.ReplyParameters().StringValue("reply_command")
	s().NoError(err)
	s().Equal(test.units[i].Command, onReplyCmd)

	// Clean out
	mockedManagerClient, err := manager_client.New(mockedConfig)
	s().NoError(err)

	err = mockedManagerClient.Close()
	s().NoError(err)

	err = proxy.ctx.Close()
	s().NoError(err)

	parent0Client, err := manager_client.New(test.handlerConfigs[0])
	s().NoError(err)
	err = parent0Client.Close()
	s().NoError(err)

	parent1Client, err := manager_client.New(test.handlerConfigs[1])
	s().NoError(err)
	err = parent1Client.Close()
	s().NoError(err)

	parent2Client, err := manager_client.New(test.handlerConfigs[2])
	s().NoError(err)
	err = parent2Client.Close()
	s().NoError(err)

	err = client1.Close()
	s().NoError(err)
	err = client2.Close()
	s().NoError(err)
	err = client3.Close()
	s().NoError(err)

	proxyHandler1, err := manager_client.New(proxyHandlerConf1)
	s().NoError(err)
	err = proxyHandler1.Close()
	s().NoError(err)

	proxyHandler2, err := manager_client.New(proxyHandlerConf2)
	s().NoError(err)
	err = proxyHandler2.Close()
	s().NoError(err)

	proxyHandler3, err := manager_client.New(proxyHandlerConf3)
	s().NoError(err)
	err = proxyHandler3.Close()
	s().NoError(err)

	// Wait a bit for closing the threads
	time.Sleep(time.Millisecond * 100)
}

func TestProxy(t *testing.T) {
	suite.Run(t, new(TestProxySuite))
}
