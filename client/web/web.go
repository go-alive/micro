// Package web is a web dashboard
package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-alive/cli"
	"github.com/go-alive/go-micro"
	res "github.com/go-alive/go-micro/api/resolver"
	"github.com/go-alive/go-micro/api/server"
	"github.com/go-alive/go-micro/api/server/cors"
	httpapi "github.com/go-alive/go-micro/api/server/http"
	"github.com/go-alive/go-micro/client/selector"
	"github.com/go-alive/go-micro/config/cmd"
	log "github.com/go-alive/go-micro/logger"
	"github.com/go-alive/go-micro/registry"
	"github.com/go-alive/micro/internal/handler"
	"github.com/go-alive/micro/internal/namespace"
	"github.com/go-alive/micro/internal/resolver/web"
	"github.com/go-alive/micro/internal/stats"
	"github.com/go-alive/micro/plugin"
	"github.com/gorilla/mux"
	"github.com/serenize/snaker"
	"golang.org/x/net/publicsuffix"
)

//Meta Fields of micro web
var (
	// Default server name
	Name = "go.micro.web"
	// Default address to bind to
	Address = ":8082"
	// The namespace to serve
	// Example:
	// Namespace + /[Service]/foo/bar
	// Host: Namespace.Service Endpoint: /foo/bar
	Namespace = "go.micro"
	Type      = "web"
	Resolver  = "path"
	// Base path sent to web service.
	// This is stripped from the request path
	// Allows the web service to define absolute paths
	BasePathHeader = "X-Micro-Web-Base-Path"
	statsURL       string
	loginURL       string

	// Host name the web dashboard is served on
	Host, _ = os.Hostname()
)

type srv struct {
	*mux.Router
	// registry we use
	registry registry.Registry
	// the resolver
	resolver *web.Resolver
	// the namespace resolver
	nsResolver *namespace.Resolver
	// the proxy server
	prx *proxy
}

type reg struct {
	registry.Registry

	sync.RWMutex
	lastPull time.Time
	services []*registry.Service
}

// ServeHTTP serves the web dashboard and proxies where appropriate
func (s *srv) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Host) == 0 {
		r.URL.Host = r.Host
	}

	if len(r.URL.Scheme) == 0 {
		r.URL.Scheme = "http"
	}

	// no host means dashboard
	host := r.URL.Hostname()
	if len(host) == 0 {
		h, _, err := net.SplitHostPort(r.Host)
		if err != nil && strings.Contains(err.Error(), "missing port in address") {
			host = r.Host
		} else if err == nil {
			host = h
		}
	}

	// check again
	if len(host) == 0 {
		s.Router.ServeHTTP(w, r)
		return
	}

	// check based on host set
	if len(Host) > 0 && Host == host {
		s.Router.ServeHTTP(w, r)
		return
	}

	// an ip instead of hostname means dashboard
	ip := net.ParseIP(host)
	if ip != nil {
		s.Router.ServeHTTP(w, r)
		return
	}

	// namespace matching host means dashboard
	parts := strings.Split(host, ".")
	reverse(parts)
	namespace := strings.Join(parts, ".")

	// replace mu since we know its ours
	if strings.HasPrefix(namespace, "mu.micro") {
		namespace = strings.Replace(namespace, "mu.micro", "go.micro", 1)
	}

	// web dashboard if namespace matches
	if namespace == Namespace+"."+Type {
		s.Router.ServeHTTP(w, r)
		return
	}

	// if a host has no subdomain serve dashboard
	v, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil || v == host {
		s.Router.ServeHTTP(w, r)
		return
	}

	// check if its a web request
	if _, _, isWeb := s.resolver.Info(r); isWeb {
		s.Router.ServeHTTP(w, r)
		return
	}

	// otherwise serve the proxy
	s.prx.ServeHTTP(w, r)
}

// proxy is a http reverse proxy
func (s *srv) proxy() *proxy {
	director := func(r *http.Request) {
		kill := func() {
			r.URL.Host = ""
			r.URL.Path = ""
			r.URL.Scheme = ""
			r.Host = ""
			r.RequestURI = ""
		}

		// TODO: better error handling
		// check to see if the endpoint was encoded in the request context
		// by the auth wrapper
		var endpoint *res.Endpoint
		if val, ok := (r.Context().Value(res.Endpoint{})).(*res.Endpoint); ok {
			endpoint = val
		}
		var err error
		if endpoint == nil {
			if endpoint, err = s.resolver.Resolve(r); err != nil {
				fmt.Printf("Failed to resolve url: %v: %v\n", r.URL, err)
				kill()
				return
			}
		}

		r.Header.Set(BasePathHeader, "/"+endpoint.Name)
		r.URL.Host = endpoint.Host
		r.URL.Path = endpoint.Path
		r.URL.Scheme = "http"
		r.Host = r.URL.Host
	}

	return &proxy{
		Router:   &httputil.ReverseProxy{Director: director},
		Director: director,
	}
}

func format(v *registry.Value) string {
	if v == nil || len(v.Values) == 0 {
		return "{}"
	}
	var f []string
	for _, k := range v.Values {
		f = append(f, formatEndpoint(k, 0))
	}
	return fmt.Sprintf("{\n%s}", strings.Join(f, ""))
}

func formatEndpoint(v *registry.Value, r int) string {
	// default format is tabbed plus the value plus new line
	fparts := []string{"", "%s %s", "\n"}
	for i := 0; i < r+1; i++ {
		fparts[0] += "\t"
	}
	// its just a primitive of sorts so return
	if len(v.Values) == 0 {
		return fmt.Sprintf(strings.Join(fparts, ""), snaker.CamelToSnake(v.Name), v.Type)
	}

	// this thing has more things, it's complex
	fparts[1] += " {"

	vals := []interface{}{snaker.CamelToSnake(v.Name), v.Type}

	for _, val := range v.Values {
		fparts = append(fparts, "%s")
		vals = append(vals, formatEndpoint(val, r+1))
	}

	// at the end
	l := len(fparts) - 1
	for i := 0; i < r+1; i++ {
		fparts[l] += "\t"
	}
	fparts = append(fparts, "}\n")

	return fmt.Sprintf(strings.Join(fparts, ""), vals...)
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	return
}

func (s *srv) indexHandler(w http.ResponseWriter, r *http.Request) {
	cors.SetHeaders(w, r)

	if r.Method == "OPTIONS" {
		return
	}

	services, err := s.registry.ListServices(registry.ListContext(r.Context()))
	if err != nil {
		log.Errorf("Error listing services: %v", err)
	}

	type webService struct {
		Name string
		Link string
		Icon string // TODO: lookup icon
	}

	// if the resolver is subdomain, we will need the domain
	domain, _ := publicsuffix.EffectiveTLDPlusOne(r.URL.Hostname())

	var webServices []webService
	for _, srv := range services {
		// not a web app
		comps := strings.Split(srv.Name, ".web.")
		if len(comps) == 1 {
			continue
		}
		name := comps[1]

		link := fmt.Sprintf("/%v/", name)
		if Resolver == "subdomain" && len(domain) > 0 {
			link = fmt.Sprintf("https://%v.%v", name, domain)
		}

		// in the case of 3 letter things e.g m3o convert to M3O
		if len(name) <= 3 && strings.ContainsAny(name, "012345789") {
			name = strings.ToUpper(name)
		}

		webServices = append(webServices, webService{Name: name, Link: link})
	}

	sort.Slice(webServices, func(i, j int) bool { return webServices[i].Name < webServices[j].Name })

	type templateData struct {
		HasWebServices bool
		WebServices    []webService
	}

	data := templateData{len(webServices) > 0, webServices}
	s.render(w, r, indexTemplate, data)
}

func (s *srv) registryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	svc := vars["name"]

	if len(svc) > 0 {
		sv, err := s.registry.GetService(svc, registry.GetContext(r.Context()))
		if err != nil {
			http.Error(w, "Error occurred:"+err.Error(), 500)
			return
		}

		if len(sv) == 0 {
			http.Error(w, "Not found", 404)
			return
		}

		if r.Header.Get("Content-Type") == "application/json" {
			b, err := json.Marshal(map[string]interface{}{
				"services": s,
			})
			if err != nil {
				http.Error(w, "Error occurred:"+err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
			return
		}

		s.render(w, r, serviceTemplate, sv)
		return
	}

	services, err := s.registry.ListServices(registry.ListContext(r.Context()))
	if err != nil {
		log.Errorf("Error listing services: %v", err)
	}

	sort.Sort(sortedServices{services})

	if r.Header.Get("Content-Type") == "application/json" {
		b, err := json.Marshal(map[string]interface{}{
			"services": services,
		})
		if err != nil {
			http.Error(w, "Error occurred:"+err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
		return
	}

	s.render(w, r, registryTemplate, services)
}

func (s *srv) callHandler(w http.ResponseWriter, r *http.Request) {
	services, err := s.registry.ListServices(registry.ListContext(r.Context()))
	if err != nil {
		log.Errorf("Error listing services: %v", err)
	}

	sort.Sort(sortedServices{services})

	serviceMap := make(map[string][]*registry.Endpoint)
	for _, service := range services {
		if len(service.Endpoints) > 0 {
			serviceMap[service.Name] = service.Endpoints
			continue
		}
		// lookup the endpoints otherwise
		s, err := s.registry.GetService(service.Name, registry.GetContext(r.Context()))
		if err != nil {
			continue
		}
		if len(s) == 0 {
			continue
		}
		serviceMap[service.Name] = s[0].Endpoints
	}

	if r.Header.Get("Content-Type") == "application/json" {
		b, err := json.Marshal(map[string]interface{}{
			"services": services,
		})
		if err != nil {
			http.Error(w, "Error occurred:"+err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
		return
	}

	s.render(w, r, callTemplate, serviceMap)
}

func (s *srv) render(w http.ResponseWriter, r *http.Request, tmpl string, data interface{}) {
	t, err := template.New("template").Funcs(template.FuncMap{
		"format": format,
		"Title":  strings.Title,
		"First": func(s string) string {
			if len(s) == 0 {
				return s
			}
			return strings.Title(string(s[0]))
		},
	}).Parse(layoutTemplate)
	if err != nil {
		http.Error(w, "Error occurred:"+err.Error(), 500)
		return
	}
	t, err = t.Parse(tmpl)
	if err != nil {
		http.Error(w, "Error occurred:"+err.Error(), 500)
		return
	}

	// If the user is logged in, render Account instead of Login
	loginTitle := "Login"
	user := ""

	if err := t.ExecuteTemplate(w, "layout", map[string]interface{}{
		"LoginTitle": loginTitle,
		"LoginURL":   loginURL,
		"StatsURL":   statsURL,
		"Results":    data,
		"User":       user,
	}); err != nil {
		http.Error(w, "Error occurred:"+err.Error(), 500)
	}
}

func Run(ctx *cli.Context, srvOpts ...micro.Option) {
	log.Init(log.WithFields(map[string]interface{}{"service": "web"}))

	if len(ctx.String("server_name")) > 0 {
		Name = ctx.String("server_name")
	}
	if len(ctx.String("address")) > 0 {
		Address = ctx.String("address")
	}
	if len(ctx.String("resolver")) > 0 {
		Resolver = ctx.String("resolver")
	}
	if len(ctx.String("type")) > 0 {
		Type = ctx.String("type")
	}
	if len(ctx.String("namespace")) > 0 {
		// remove the service type from the namespace to allow for
		// backwards compatability
		Namespace = strings.TrimSuffix(ctx.String("namespace"), "."+Type)
	}

	// Init plugins
	for _, p := range Plugins() {
		p.Init(ctx)
	}

	// service opts
	srvOpts = append(srvOpts, micro.Name(Name))

	// Initialize Server
	service := micro.NewService(srvOpts...)

	reg := &reg{Registry: *cmd.DefaultCmd.Options().Registry}

	s := &srv{
		Router:   mux.NewRouter(),
		registry: reg,
		// our internal resolver
		resolver: &web.Resolver{
			// Default to type path
			Type:      Resolver,
			Namespace: namespace.NewResolver(Type, Namespace).ResolveWithType,
			Selector: selector.NewSelector(
				selector.Registry(reg),
			),
		},
	}

	var h http.Handler
	// set as the server
	h = s

	if ctx.Bool("enable_stats") {
		statsURL = "/stats"
		st := stats.New()
		s.HandleFunc("/stats", st.StatsHandler)
		h = st.ServeHTTP(s)
		st.Start()
		defer st.Stop()
	}

	// create the proxy
	p := s.proxy()

	// the web handler itself
	s.HandleFunc("/favicon.ico", faviconHandler)
	s.HandleFunc("/client", s.callHandler)
	s.HandleFunc("/services", s.registryHandler)
	s.HandleFunc("/service/{name}", s.registryHandler)
	s.HandleFunc("/rpc", handler.RPC)
	s.PathPrefix("/{service:[a-zA-Z0-9]+}").Handler(p)
	s.HandleFunc("/", s.indexHandler)

	// insert the proxy
	s.prx = p

	var opts []server.Option

	// reverse wrap handler
	plugins := append(Plugins(), plugin.Plugins()...)
	for i := len(plugins); i > 0; i-- {
		h = plugins[i-1].Handler()(h)
	}

	// create the namespace resolver and the auth wrapper
	s.nsResolver = namespace.NewResolver(Type, Namespace)

	// create the service and add the auth wrapper
	srv := httpapi.NewServer(Address) //, server.WrapHandler(authWrapper))

	srv.Init(opts...)
	srv.Handle("/", h)

	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}

	// Run server
	if err := service.Run(); err != nil {
		log.Fatal(err)
	}

	if err := srv.Stop(); err != nil {
		log.Fatal(err)
	}
}

//Commands for `micro web`
func Commands(options ...micro.Option) []*cli.Command {
	command := &cli.Command{
		Name:  "web",
		Usage: "Run the web dashboard",
		Action: func(c *cli.Context) error {
			Run(c, options...)
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "address",
				Usage:   "Set the web UI address e.g 0.0.0.0:8082",
				EnvVars: []string{"MICRO_WEB_ADDRESS"},
			},
			&cli.StringFlag{
				Name:    "namespace",
				Usage:   "Set the namespace used by the Web proxy e.g. com.example.web",
				EnvVars: []string{"MICRO_WEB_NAMESPACE"},
			},
			&cli.StringFlag{
				Name:    "resolver",
				Usage:   "Set the resolver to route to services e.g path, domain",
				EnvVars: []string{"MICRO_WEB_RESOLVER"},
			},
			&cli.StringFlag{
				Name:    "auth_login_url",
				EnvVars: []string{"MICRO_AUTH_LOGIN_URL"},
				Usage:   "The relative URL where a user can login",
			},
		},
	}

	for _, p := range Plugins() {
		if cmds := p.Commands(); len(cmds) > 0 {
			command.Subcommands = append(command.Subcommands, cmds...)
		}

		if flags := p.Flags(); len(flags) > 0 {
			command.Flags = append(command.Flags, flags...)
		}
	}

	return []*cli.Command{command}
}

func reverse(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
