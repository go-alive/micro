package namespace

import (
	"net"
	"net/http"
	"strings"

	"github.com/go-alive/go-micro/logger"
	"golang.org/x/net/publicsuffix"
)

func NewResolver(srvType, namespace string) *Resolver {
	return &Resolver{srvType, namespace}
}

// Resolver determines the namespace for a request
type Resolver struct {
	srvType   string
	namespace string
}

func (r Resolver) String() string {
	return "internal/namespace"
}

func (r Resolver) ResolveWithType(req *http.Request) string {
	return r.Resolve(req) + "." + r.srvType
}

func (r Resolver) Resolve(req *http.Request) string {
	// check to see what the provided namespace is, we only do
	// domain mapping if the namespace is set to 'domain'
	if r.namespace != "domain" {
		return r.namespace
	}

	// determine the host, e.g. dev.micro.mu:8080
	host := req.URL.Hostname()
	if len(host) == 0 {
		if h, _, err := net.SplitHostPort(req.Host); err == nil {
			host = h // host does contain a port
		} else if strings.Contains(err.Error(), "missing port in address") {
			host = req.Host // host does not contain a port
		}
	}

	// check for an ip address
	if net.ParseIP(host) != nil {
		return DefaultNamespace
	}

	// check for dev enviroment
	if host == "localhost" || host == "127.0.0.1" {
		return DefaultNamespace
	}

	// extract the top level domain plus one (e.g. 'myapp.com')
	domain, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		logger.Debugf("Unable to extract domain from %v", host)
		return DefaultNamespace
	}

	// check to see if the domain matches the host of micro.mu, in
	// these cases we return the default namespace
	if domain == host || domain == "micro.mu" {
		return DefaultNamespace
	}

	// remove the domain from the host, leaving the subdomain
	subdomain := strings.TrimSuffix(host, "."+domain)

	// return the reversed subdomain as the namespace
	comps := strings.Split(subdomain, ".")
	for i := len(comps)/2 - 1; i >= 0; i-- {
		opp := len(comps) - 1 - i
		comps[i], comps[opp] = comps[opp], comps[i]
	}
	return strings.Join(comps, ".")
}
