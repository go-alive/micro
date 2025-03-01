package client

import (
	"context"

	ccli "github.com/go-alive/cli"
	"github.com/go-alive/go-micro/auth"
	"github.com/go-alive/go-micro/client"
	"github.com/go-alive/go-micro/client/grpc"
	"github.com/go-alive/go-micro/metadata"
	"github.com/go-alive/micro/client/cli/util"
	cliutil "github.com/go-alive/micro/client/cli/util"
	"github.com/go-alive/micro/internal/config"
)

// New returns a wrapped grpc client which will inject the
// token found in config into each request
func New(ctx *ccli.Context) client.Client {
	env := cliutil.GetEnv(ctx)
	token, _ := config.Get("micro", "auth", env.Name, "token")
	return &wrapper{grpc.NewClient(), token, env.Name, ctx}
}

type wrapper struct {
	client.Client
	token string
	env   string
	ctx   *ccli.Context
}

func (a *wrapper) Call(ctx context.Context, req client.Request, rsp interface{}, opts ...client.CallOption) error {
	if len(a.token) > 0 {
		ctx = metadata.Set(ctx, "Authorization", auth.BearerScheme+a.token)
	}
	if len(a.env) > 0 && !util.IsLocal(a.ctx) && !util.IsServer(a.ctx) {
		// @todo this is temporarily removed because multi tenancy is not there yet
		// and the moment core and non core services run in different environments, we
		// get issues. To test after `micro env add mine 127.0.0.1:8081` do,
		// `micro run github.com/crufter/micro-services/logspammer` works but
		// `micro -env=mine run github.com/crufter/micro-services/logspammer` is broken.
		// Related ticket https://github.com/micro/development/issues/193
		//
		// env := strings.ReplaceAll(a.env, "/", "-")
		// ctx = metadata.Set(ctx, "Micro-Namespace", env)
	}
	return a.Client.Call(ctx, req, rsp, opts...)
}
