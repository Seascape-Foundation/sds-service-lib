// Package dev handles the dependencies in the development environment,
// which means it's in the current machine.
//
// The dependencies are including the extensions and proxies.
//
// How it works?
//
// The orchestra is set up. It checks the folder. And if they are not existing, it will create them.
// >> dev.Run(orchestra)
//
// then lets work on the extension.
// User is passing an extension url.
// The service is checking whether it exists in the data or not.
// If the service exists, it gets the yaml. And returns the config.
//
// If the service doesn't exist, it checks whether the service exists in the bin.
// If it exists, then it runs it with --build-config.
//
// Then, if the service doesn't exist in the bin, it checks the source.
// If the source exists, then it will call `go build`.
// Then call bin file with the generated files.
//
// Lastly, if a source doesn't exist, it will download the files from the repository using go-git.
// Then we build the binary.
// We generate config.
//
// Lastly, the service.Run() will make sure that all binaries exist.
// If not, then it will create them.
//
// -----------------------------------------------
// running the application will do the following.
// It checks the port of proxies is in use. If it's not, then it will call a run.
//
// Then it will call itself.
//
// The service will have a command to "shutdown" contexts. As well as "rebuild"
package dev

import (
	"fmt"
	"github.com/ahmetson/common-lib/data_type/key_value"
	"github.com/ahmetson/os-lib/env"
	"github.com/ahmetson/os-lib/path"
	"github.com/ahmetson/service-lib/config"
	"github.com/ahmetson/service-lib/orchestra"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

const (
	SrcKey  = "SERVICE_DEPS_SRC"
	BinKey  = "SERVICE_DEPS_BIN"
	DataKey = "SERVICE_DEPS_DATA"
)

// A Context handles the config of the contexts
type Context struct {
	Src          string `json:"SERVICE_DEPS_SRC"`
	Bin          string `json:"SERVICE_DEPS_BIN"`
	Data         string `json:"SERVICE_DEPS_DATA"`
	url          string
	controller   *handler.Controller
	serviceReady bool
	deps         map[string]*Dep
}

// Creates the directory for the data at the given path.
func preparePath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(path, 0777); err != nil {
				return fmt.Errorf("failed to create a directory at '%s' path: %w", path, err)
			}
			return nil
		} else {
			return fmt.Errorf("failed to read '%s': %w", path, err)
		}
	}

	if !info.IsDir() {
		return fmt.Errorf("the path '%s' is not a directory", path)
	}

	return nil
}

// New creates an orchestra including its directories.
func New(conf *config.Service) (*Context, error) {
	execPath, err := path.GetExecPath()
	if err != nil {
		return nil, fmt.Errorf("path.GetExecPath: %w", err)
	}
	srcPath := path.GetPath(execPath, conf.Parent().Engine().GetString(SrcKey))
	dataPath := path.GetPath(execPath, conf.Parent().Engine().GetString(DataKey))
	binPath := path.GetPath(execPath, conf.Parent().Engine().GetString(BinKey))

	ctx := &Context{
		Src:        srcPath,
		Bin:        binPath,
		Data:       dataPath,
		deps:       make(map[string]*Dep),
		url:        conf.Url,
		controller: nil,
	}

	if err != nil {
		return nil, fmt.Errorf("newContext: %w", err)
	}

	for _, path := range ctx.Paths() {
		if err := preparePath(path); err != nil {
			return ctx, fmt.Errorf("preparePath(%s): %w", path, err)
		}
	}

	if err := ctx.prepareEnv(); err != nil {
		return ctx, fmt.Errorf("prepareEnv: %w", err)
	}

	return ctx, nil
}

// prepareEnv writes the .env of the orchestra to share between dependencies.
// Call it after creating a path.
func (context *Context) prepareEnv() error {
	kv, err := key_value.NewFromInterface(context)
	if err != nil {
		return fmt.Errorf("prepareEnv: %w", err)
	}

	err = env.WriteEnv(kv, context.EnvPath())
	if err != nil {
		return fmt.Errorf("env.WriteEnv: %w", err)
	}

	return nil
}

// Dep returns the dependency from the orchestra by its url.
// Returns error, if the dependency wasn't found
func (context *Context) Dep(url string) (*Dep, error) {
	dep, ok := context.deps[url]
	if !ok {
		return nil, fmt.Errorf("the '%s' dependency not exists", url)
	}
	return dep, nil
}

// New dependency in the orchestra. If the dependency already exists, it will return an error.
// The created dependency will be added to the orchestra.
func (context *Context) New(url string) (*Dep, error) {
	_, ok := context.deps[url]
	if ok {
		return nil, fmt.Errorf("the '%s' dependency exists", url)
	}
	dep := &Dep{url: url, context: context, cmd: nil}
	context.deps[url] = dep
	return dep, nil
}

func (c *Context) GetType() orchestra.Type {
	return orchestra.DevContext
}

// GetConfig on the given path.
// If a path is not obsolete, then it should be relative to the executable.
// The path should have the .yml extension
func (c *Context) GetConfig(url string) (*config.Service, error) {
	path := c.ConfigurationPath(url)

	if err := validateServicePath(path); err != nil {
		return nil, fmt.Errorf("validateServicePath: %w", err)
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("os.ReadFile of %s: %w", path, err)
	}

	yamlConfig := createYaml()
	kv := yamlConfig.Map()
	err = yaml.Unmarshal(bytes, &kv)

	if err != nil {
		return nil, fmt.Errorf("yaml.Unmarshal of %s: %w", path, err)
	}

	fmt.Println("service", kv)

	yamlConfig = key_value.New(kv)
	if err := yamlConfig.Exist("Services"); err != nil {
		return nil, fmt.Errorf("no services in yaml: %w", err)
	}

	services, err := yamlConfig.GetKeyValueList("Services")
	if err != nil {
		return nil, fmt.Errorf("failed to get services as key value list: %w", err)
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("no services in the config")
	}

	var serviceConfig config.Service
	err = services[0].Interface(&serviceConfig)
	if err != nil {
		return nil, fmt.Errorf("convert key value to Service: %w", err)
	}

	err = serviceConfig.PrepareService()
	if err != nil {
		return nil, fmt.Errorf("prepareService: %w", err)
	}

	return &serviceConfig, nil
}

// WriteService writes the service as the yaml on the given path.
// If the path doesn't contain the file extension, it will through an error
func (c *Context) SetConfig(url string, service *config.Service) error {
	path := c.ConfigurationPath(url)

	if err := validateServicePath(path); err != nil {
		return fmt.Errorf("validateServicePath: %w", err)
	}

	kv := createYaml(service)

	serviceConfig, err := yaml.Marshal(kv.Map())
	if err != nil {
		return fmt.Errorf("failed to marshall config.Service: %w", err)
	}

	f, _ := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	_, err = f.Write(serviceConfig)
	closeErr := f.Close()
	if err != nil {
		return fmt.Errorf("failed to write service into the given path: %w", err)
	} else if closeErr != nil {
		return fmt.Errorf("failed to close the file descriptor: %w", closeErr)
	} else {
		return nil
	}
}

// validateServicePath returns an error if the path is not a valid .yml link
func validateServicePath(path string) error {
	if len(path) < 5 || len(filepath.Base(path)) < 5 {
		return fmt.Errorf("path is too short")
	}
	_, found := strings.CutSuffix(path, ".yml")
	if !found {
		return fmt.Errorf("the path should end with '.yml'")
	}

	return nil
}

func createYaml(configs ...*config.Service) key_value.KeyValue {
	var services = configs
	kv := key_value.Empty()
	kv.Set("Services", services)

	return kv
}

func (c *Context) Paths() []string {
	return []string{c.Data, c.Bin, c.Src}
}

func (c *Context) SetUrl(url string) {
	c.url = url
}

func (c *Context) GetUrl() string {
	return c.url
}

func (c *Context) Host() string {
	return "localhost"
}

//----------------------------------------------------------
// below are the specific functions for the dev orchestra. other contexts may not have them
//----------------------------------------------------------

// EnvPath is the shared configurations between dependencies
func (c *Context) EnvPath() string {
	return filepath.Join(c.Data, ".env")
}

// ConfigurationPath returns config url in the orchestra's data
func (c *Context) ConfigurationPath(url string) string {
	fileName := config.UrlToFileName(c.GetUrl())
	return filepath.Join(c.Data, url, fileName+".yml")
}

func (c *Context) SrcPath(url string) string {
	return filepath.Join(c.Src, url)
}
