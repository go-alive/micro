package cmd

import (
	"fmt"
	"os"
	"os/exec"

	ccli "github.com/go-alive/cli"
	"github.com/go-alive/go-micro"
	"github.com/go-alive/go-micro/config/cmd"
	"github.com/go-alive/micro/plugin"

	// clients
	"github.com/go-alive/micro/client/api"
	"github.com/go-alive/micro/client/cli"
	"github.com/go-alive/micro/client/cli/new"
	"github.com/go-alive/micro/client/web"

	// internals
	"github.com/go-alive/micro/internal/helper"
)

var (
	GitCommit string
	GitTag    string
	BuildDate string

	name        = "micro"
	description = "A microservice runtime"
	version     = "latest"
)

func init() {
}

func setup(app *ccli.App) {
	app.Flags = append(app.Flags,
		&ccli.BoolFlag{
			Name:    "enable_tls",
			Usage:   "Enable TLS support. Expects cert and key file to be specified",
			EnvVars: []string{"MICRO_ENABLE_TLS"},
		},
		&ccli.StringFlag{
			Name:    "tls_cert_file",
			Usage:   "Path to the TLS Certificate file",
			EnvVars: []string{"MICRO_TLS_CERT_FILE"},
		},
		&ccli.StringFlag{
			Name:    "tls_key_file",
			Usage:   "Path to the TLS Key file",
			EnvVars: []string{"MICRO_TLS_KEY_FILE"},
		},
		&ccli.StringFlag{
			Name:    "tls_client_ca_file",
			Usage:   "Path to the TLS CA file to verify clients against",
			EnvVars: []string{"MICRO_TLS_CLIENT_CA_FILE"},
		},
		&ccli.StringFlag{
			Name:    "api_address",
			Usage:   "Set the api address e.g 0.0.0.0:8080",
			EnvVars: []string{"MICRO_API_ADDRESS"},
		},
		&ccli.StringFlag{
			Name:    "namespace",
			Usage:   "Set the micro service namespace",
			EnvVars: []string{"MICRO_NAMESPACE"},
			Value:   "micro",
		},
		&ccli.StringFlag{
			Name:    "web_address",
			Usage:   "Set the web UI address e.g 0.0.0.0:8082",
			EnvVars: []string{"MICRO_WEB_ADDRESS"},
		},
		&ccli.StringFlag{
			Name:    "network",
			Usage:   "Set the micro network name: local, go.micro",
			EnvVars: []string{"MICRO_NETWORK"},
		},
		&ccli.StringFlag{
			Name:    "network_address",
			Usage:   "Set the micro network address e.g. :9093",
			EnvVars: []string{"MICRO_NETWORK_ADDRESS"},
		},
		&ccli.StringFlag{
			Name:    "gateway_address",
			Usage:   "Set the micro default gateway address e.g. :9094",
			EnvVars: []string{"MICRO_GATEWAY_ADDRESS"},
		},
		&ccli.StringFlag{
			Name:    "api_handler",
			Usage:   "Specify the request handler to be used for mapping HTTP requests to services; {api, rpc}",
			EnvVars: []string{"MICRO_API_HANDLER"},
		},
		&ccli.StringFlag{
			Name:    "api_namespace",
			Usage:   "Set the namespace used by the API e.g. com.example.api",
			EnvVars: []string{"MICRO_API_NAMESPACE"},
		},
		&ccli.StringFlag{
			Name:    "web_namespace",
			Usage:   "Set the namespace used by the Web proxy e.g. com.example.web",
			EnvVars: []string{"MICRO_WEB_NAMESPACE"},
		},
		&ccli.StringFlag{
			Name:    "web_url",
			Usage:   "Set the host used for the web dashboard e.g web.example.com",
			EnvVars: []string{"MICRO_WEB_HOST"},
		},
		&ccli.BoolFlag{
			Name:    "enable_stats",
			Usage:   "Enable stats",
			EnvVars: []string{"MICRO_ENABLE_STATS"},
		},
		&ccli.StringFlag{
			Name:    "env",
			Aliases: []string{"e"},
			Usage:   "Override environment",
			EnvVars: []string{"MICRO_ENV"},
		},
	)

	plugins := plugin.Plugins()

	for _, p := range plugins {
		if flags := p.Flags(); len(flags) > 0 {
			app.Flags = append(app.Flags, flags...)
		}

		if cmds := p.Commands(); len(cmds) > 0 {
			app.Commands = append(app.Commands, cmds...)
		}
	}

	before := app.Before

	app.Before = func(ctx *ccli.Context) error {

		if len(ctx.String("api_handler")) > 0 {
			api.Handler = ctx.String("api_handler")
		}
		if len(ctx.String("api_address")) > 0 {
			api.Address = ctx.String("api_address")
		}
		if len(ctx.String("web_address")) > 0 {
			web.Address = ctx.String("web_address")
		}
		if len(ctx.String("api_namespace")) > 0 {
			api.Namespace = ctx.String("api_namespace")
		}
		if len(ctx.String("web_namespace")) > 0 {
			web.Namespace = ctx.String("web_namespace")
		}
		if len(ctx.String("web_host")) > 0 {
			web.Host = ctx.String("web_host")
		}

		for _, p := range plugins {
			if err := p.Init(ctx); err != nil {
				return err
			}
		}

		// now do previous before
		if err := before(ctx); err != nil {
			// DO NOT return this error otherwise the action will fail
			// and help will be printed.
			fmt.Println(err)
			os.Exit(1)
			return err
		}

		return nil
	}
}

func buildVersion() string {
	microVersion := version

	if GitTag != "" {
		microVersion = GitTag
	}

	if GitCommit != "" {
		microVersion += fmt.Sprintf("-%s", GitCommit)
	}

	if BuildDate != "" {
		microVersion += fmt.Sprintf("-%s", BuildDate)
	}

	return microVersion
}

// Init initialised the command line
func Init(options ...micro.Option) {
	Setup(cmd.App(), options...)

	cmd.Init(
		cmd.Name(name),
		cmd.Description(description),
		cmd.Version(buildVersion()),
	)
}

// Setup sets up a cli.App
func Setup(app *ccli.App, options ...micro.Option) {
	// Add the various commands
	app.Commands = append(app.Commands, api.Commands(options...)...)
	app.Commands = append(app.Commands, cli.Commands()...)
	app.Commands = append(app.Commands, new.Commands()...)
	app.Commands = append(app.Commands, web.Commands(options...)...)

	// boot micro runtime
	app.Action = func(c *ccli.Context) error {
		if c.Args().Len() > 0 {
			command := c.Args().First()

			v, err := exec.LookPath(command)
			if err != nil {
				fmt.Println(helper.UnexpectedCommand(c))
				os.Exit(1)
			}

			// execute the command
			ce := exec.Command(v, c.Args().Slice()[1:]...)
			ce.Stdout = os.Stdout
			ce.Stderr = os.Stderr
			return ce.Run()
		}
		fmt.Println(helper.MissingCommand(c))
		os.Exit(1)
		return nil
	}

	setup(app)
}
