package service

import (
	"fmt"
)

// Proxy defines the parameters of the proxy service
type Proxy struct {
	*Service
}

// NewProxy proxy service returned
func NewProxy() (*Proxy, error) {
	independent, err := New()
	if err != nil {
		return nil, fmt.Errorf("service.New: %w", err)
	}

	return &Proxy{independent}, nil
}
