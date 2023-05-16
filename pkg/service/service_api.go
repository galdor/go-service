package service

import (
	"net/http"
	"net/http/pprof"

	jsonvalidator "github.com/galdor/go-json-validator"
	"github.com/galdor/go-log"
	"github.com/galdor/go-service/pkg/shttp"
)

type ServiceAPICfg struct {
	Log     *log.Logger `json:"-"`
	Service *Service    `json:"-"`

	HTTPServer string `json:"httpServer"`
}

type ServiceAPI struct {
	Cfg     ServiceAPICfg
	Service *Service
	Log     *log.Logger

	HTTPServer *shttp.Server
}

func (cfg *ServiceAPICfg) ValidateJSON(v *jsonvalidator.Validator) {
	v.CheckStringNotEmpty("httpServer", cfg.HTTPServer)
}

func NewServiceAPI(cfg ServiceAPICfg) *ServiceAPI {
	httpServerName := cfg.HTTPServer
	httpServer := cfg.Service.HTTPServer(httpServerName)

	// The pprof module breaks if we redirect /debug/pprof/ to /debug/pprof.
	httpServer.Cfg.DisableTrailingSlashHandling = true

	return &ServiceAPI{
		Cfg:     cfg,
		Service: cfg.Service,
		Log:     cfg.Log,

		HTTPServer: httpServer,
	}
}

func (s *ServiceAPI) Start() error {
	s.initRoutes(s.HTTPServer)

	return nil
}

func (s *ServiceAPI) initRoutes(server *shttp.Server) {
	handlerFunc := func(handler http.Handler) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			handler.ServeHTTP(w, req)
		}
	}

	wrap := func(fn http.HandlerFunc) shttp.RouteFunc {
		return func(h *shttp.Handler) {
			fn(h.ResponseWriter, h.Request)
		}
	}

	routes := map[string]http.HandlerFunc{
		"/cmdline": pprof.Cmdline,
		"/profile": pprof.Profile,
		"/symbol":  pprof.Symbol,
		"/trace":   pprof.Trace,

		"/allocs":       handlerFunc(pprof.Handler("allocs")),
		"/block":        handlerFunc(pprof.Handler("block")),
		"/goroutine":    handlerFunc(pprof.Handler("goroutine")),
		"/heap":         handlerFunc(pprof.Handler("heap")),
		"/mutex":        handlerFunc(pprof.Handler("mutex")),
		"/threadcreate": handlerFunc(pprof.Handler("threadcreate")),
	}

	// It would convenient to serve pprof routes at /pprof, but pprof assumes
	// that the URI starts with /debug/pprof/ (not the final "/").

	server.Route("/debug/pprof/", "GET", wrap(pprof.Index))

	for subpath, handler := range routes {
		server.Route("/debug/pprof"+subpath, "GET", wrap(handler))
	}
}

func (s *ServiceAPI) Stop() {
}
