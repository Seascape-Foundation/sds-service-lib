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
type TestAuxiliarySuite struct {
	suite.Suite

	parent     *Service // the manager to test
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

func DeleteLastFlags(amount int) {
	win.Args = win.Args[:len(win.Args)-amount]
}

func (test *TestAuxiliarySuite) SetupTest() {
	s := test.Suite.Require

	currentDir, err := path.CurrentDir()
	s().NoError(err)
	test.currentDir = currentDir

	// A valid source code that we want to download
	test.url = "github.com/ahmetson/parent-lib"
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

func (test *TestAuxiliarySuite) TearDownTest() {
	s := test.Suite.Require

	err := win.Remove(test.envPath)
	s().NoError(err, "delete the dump file: "+test.envPath)
}

// Test_10_NewAuxiliary tests NewAuxiliary
func (test *TestAuxiliarySuite) Test_10_NewAuxiliary() {
	s := test.Suite.Require

	// Creating an Auxiliary must fail since no Parent flag
	_, err := NewAuxiliary()
	s().Error(err)

	// Creating an auxiliary must fail since parent is not a valid config
	win.Args = append(win.Args, arg.NewFlag(flag.ParentFlag, "parent"))
	_, err = NewAuxiliary()
	s().Error(err)
	DeleteLastFlags(1)

	// Creating an auxiliary with the valid flags must succeed
	parentClient := clientConfig.New(test.url+"_parent", test.id+"_parent", 6000, zmq4.REP)
	parentKv, err := key_value.NewFromInterface(parentClient)
	s().NoError(err)
	parentStr := parentKv.String()
	win.Args = append(win.Args,
		arg.NewFlag(flag.IdFlag, test.id),
		arg.NewFlag(flag.UrlFlag, test.url),
		arg.NewFlag(flag.ParentFlag, parentStr),
	)

	auxiliary, err := NewAuxiliary()
	s().NoError(err)

	DeleteLastFlags(3)

	s().NoError(auxiliary.ctx.Close())
}

func TestAuxiliary(t *testing.T) {
	suite.Run(t, new(TestAuxiliarySuite))
}
