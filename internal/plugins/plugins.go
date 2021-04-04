// Package plugins includes the plugins we want to load
package plugins

import (
	"github.com/go-alive/go-micro/config/cmd"

	// import specific plugins
	ckStore "github.com/go-alive/go-micro/store/cockroach"
	fileStore "github.com/go-alive/go-micro/store/file"
	memStore "github.com/go-alive/go-micro/store/memory"
	// we only use CF internally for certs
	cfStore "github.com/go-alive/micro/internal/plugins/store/cloudflare"
)

func init() {
	// TODO: make it so we only have to import them
	cmd.DefaultStores["cloudflare"] = cfStore.NewStore
	cmd.DefaultStores["cockroach"] = ckStore.NewStore
	cmd.DefaultStores["file"] = fileStore.NewStore
	cmd.DefaultStores["memory"] = memStore.NewStore
}
