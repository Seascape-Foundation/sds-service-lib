package service

import (
	"fmt"
	clientConfig "github.com/ahmetson/client-lib/config"
	"github.com/ahmetson/datatype-lib/data_type/key_value"
	"github.com/ahmetson/os-lib/arg"
	"github.com/ahmetson/service-lib/flag"
	"github.com/ahmetson/service-lib/manager"
)

type Auxiliary struct {
	*Service
	ParentManager *manager.Client // parent to work with
}

// NewAuxiliary creates a parent with the parent.
// It requires a parent flag
func NewAuxiliary() (*Auxiliary, error) {
	if !arg.FlagExist(flag.ParentFlag) {
		return nil, fmt.Errorf("missing %s flag", arg.NewFlag(flag.ParentFlag))
	}

	//
	// Parent config in a raw string format
	//
	parentStr := arg.FlagValue(flag.ParentFlag)
	parentKv, err := key_value.NewFromString(parentStr)
	if err != nil {
		return nil, fmt.Errorf("key_value.NewFromString('%s'): %w", flag.ParentFlag, err)
	}

	//
	// Parent config
	//
	var parentConfig clientConfig.Client
	err = parentKv.Interface(&parentConfig)
	if err != nil {
		return nil, fmt.Errorf("parentKv.Interface: %w", err)
	}
	if len(parentConfig.Id) == 0 {
		return nil, fmt.Errorf("empty parent")
	}
	parentConfig.UrlFunc(clientConfig.Url)

	//
	// Parent client
	//
	parent, err := manager.NewClient(&parentConfig)
	if err != nil {
		return nil, fmt.Errorf("manager.NewClient('parentConfig'): %w", err)
	}

	independent, err := New()
	if err != nil {
		return nil, fmt.Errorf("new independent parent: %w", err)
	}

	return &Auxiliary{Service: independent, ParentManager: parent}, nil
}
