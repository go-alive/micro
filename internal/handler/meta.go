package handler

import (
	"net/http"

	"github.com/go-alive/go-micro"
	"github.com/go-alive/go-micro/api/handler"
	"github.com/go-alive/go-micro/api/handler/event"
	"github.com/go-alive/go-micro/api/router"
	"github.com/go-alive/go-micro/client"
	"github.com/go-alive/go-micro/errors"

	// TODO: only import handler package
	aapi "github.com/go-alive/go-micro/api/handler/api"
	ahttp "github.com/go-alive/go-micro/api/handler/http"
	arpc "github.com/go-alive/go-micro/api/handler/rpc"
	aweb "github.com/go-alive/go-micro/api/handler/web"
)

type metaHandler struct {
	c  client.Client
	r  router.Router
	ns func(*http.Request) string
}

func (m *metaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	service, err := m.r.Route(r)
	if err != nil {
		er := errors.InternalServerError(m.ns(r), err.Error())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write([]byte(er.Error()))
		return
	}

	// TODO: don't do this ffs
	switch service.Endpoint.Handler {
	// web socket handler
	case aweb.Handler:
		aweb.WithService(service, handler.WithClient(m.c)).ServeHTTP(w, r)
	// proxy handler
	case "proxy", ahttp.Handler:
		ahttp.WithService(service, handler.WithClient(m.c)).ServeHTTP(w, r)
	// rpcx handler
	case arpc.Handler:
		arpc.WithService(service, handler.WithClient(m.c)).ServeHTTP(w, r)
	// event handler
	case event.Handler:
		ev := event.NewHandler(
			handler.WithNamespace(m.ns(r)),
			handler.WithClient(m.c),
		)
		ev.ServeHTTP(w, r)
	// api handler
	case aapi.Handler:
		aapi.WithService(service, handler.WithClient(m.c)).ServeHTTP(w, r)
	// default handler: rpc
	default:
		arpc.WithService(service, handler.WithClient(m.c)).ServeHTTP(w, r)
	}
}

// Meta is a http.Handler that routes based on endpoint metadata
func Meta(s micro.Service, r router.Router, ns func(*http.Request) string) http.Handler {
	return &metaHandler{
		c:  s.Client(),
		r:  r,
		ns: ns,
	}
}
