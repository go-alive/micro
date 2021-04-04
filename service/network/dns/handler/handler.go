// Package handler is the handler for the `micro network dns` command
package handler

import (
	"github.com/go-alive/micro/service/network/dns/provider"
)

// New returns a new handler
func New(provider provider.Provider, token string) *DNS {
	return &DNS{
		provider:    provider,
		bearerToken: token,
	}
}
