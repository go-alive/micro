package helper

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/go-alive/cli"
	"github.com/go-alive/go-micro/metadata"
)

func ACMEHosts(ctx *cli.Context) []string {
	var hosts []string
	for _, host := range strings.Split(ctx.String("acme_hosts"), ",") {
		if len(host) > 0 {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

func RequestToContext(r *http.Request) context.Context {
	ctx := context.Background()
	md := make(metadata.Metadata)
	for k, v := range r.Header {
		md[k] = strings.Join(v, ",")
	}
	return metadata.NewContext(ctx, md)
}

func TLSConfig(ctx *cli.Context) (*tls.Config, error) {
	cert := ctx.String("tls_cert_file")
	key := ctx.String("tls_key_file")
	ca := ctx.String("tls_client_ca_file")

	if len(cert) > 0 && len(key) > 0 {
		certs, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return nil, err
		}

		if len(ca) > 0 {
			caCert, err := ioutil.ReadFile(ca)
			if err != nil {
				return nil, err
			}

			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)

			return &tls.Config{
				Certificates: []tls.Certificate{certs},
				ClientCAs:    caCertPool,
				ClientAuth:   tls.RequireAndVerifyClientCert,
				NextProtos:   []string{"h2", "http/1.1"},
			}, nil
		}

		return &tls.Config{
			Certificates: []tls.Certificate{certs}, NextProtos: []string{"h2", "http/1.1"},
		}, nil
	}

	return nil, errors.New("TLS certificate and key files not specified")
}

// UnexpectedSubcommand checks for erroneous subcommands and prints help and returns error
func UnexpectedSubcommand(ctx *cli.Context) error {
	if first := ctx.Args().First(); first != "" {
		// received something that isn't a subcommand
		return fmt.Errorf("Unrecognized subcommand for %s: %s. Please refer to '%s help'", ctx.App.Name, first, ctx.App.Name)
	}
	return nil
}

func UnexpectedCommand(ctx *cli.Context) error {
	commandName := ""
	// We fall back to os.Args as ctx does not seem to have the original command.
	for _, arg := range os.Args[1:] {
		// Exclude flags
		if !strings.HasPrefix(arg, "-") {
			commandName = arg
		}
	}
	return fmt.Errorf("Unrecognized micro command: %s. Please refer to 'micro help'", commandName)
}

func MissingCommand(ctx *cli.Context) error {
	return fmt.Errorf("No command provided to micro. Please refer to 'micro help'")
}
