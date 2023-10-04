// Package proxy defines the script that acts as the middleware
package proxy

import (
	"fmt"
	"github.com/ahmetson/service-lib"
)

// Proxy defines the parameters of the proxy service
type Proxy struct {
	*service.Service
}

// New proxy service returned
func New() (*Proxy, error) {
	independent, err := service.New()
	if err != nil {
		return nil, fmt.Errorf("service.New: %w", err)
	}

	return &Proxy{independent}, nil
}
