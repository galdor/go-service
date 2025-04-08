package shttp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.n16f.net/ejson"
	"go.n16f.net/log"
	"go.n16f.net/program"
	"go.n16f.net/service/pkg/influx"
)

type contextKey struct{}

var (
	contextKeyHandler contextKey = struct{}{}
)

type RouteFunc func(*Handler)

type ErrorData interface{}
type ErrorHandler func(*Handler, int, string, string, ErrorData)

type ServerCfg struct {
	Log           *log.Logger    `json:"-"`
	ErrorChan     chan<- error   `json:"-"`
	InfluxClient  *influx.Client `json:"-"`
	Name          string         `json:"-"`
	ErrorHandler  ErrorHandler   `json:"-"`
	DataDirectory string         `json:"-"`

	Address string `json:"address"`

	TLS *TLSServerCfg `json:"tls"`

	LogSuccessfulRequests bool `json:"log_successful_requests"`
	HideInternalErrors    bool `json:"hide_internal_errors"`
	MethodLessRouteIds    bool `json:"method_less_route_ids"`
	ShutdownTimeout       int  `json:"shutdown_timeout"` // seconds
}

type TLSServerCfg struct {
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"private_key"`
}

type Server struct {
	Cfg ServerCfg
	Log *log.Logger

	server *http.Server
	mux    *http.ServeMux

	errorHandler ErrorHandler

	errorChan chan<- error
	wg        sync.WaitGroup
}

func (cfg *ServerCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckOptionalObject("tls", cfg.TLS)
}

func (cfg *TLSServerCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckStringNotEmpty("certificate", cfg.Certificate)
	v.CheckStringNotEmpty("private_key", cfg.PrivateKey)
}

func NewServer(cfg ServerCfg) (*Server, error) {
	if cfg.Log == nil {
		cfg.Log = log.DefaultLogger("http_server")
	}

	if cfg.ErrorChan == nil {
		return nil, fmt.Errorf("missing error channel")
	}

	if cfg.Name == "" {
		return nil, fmt.Errorf("missing or empty server name")
	}

	if cfg.DataDirectory == "" {
		return nil, fmt.Errorf("missing or empty data directory")
	}

	if cfg.Address == "" {
		cfg.Address = "localhost:8080"
	}

	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = DefaultErrorHandler
	}

	s := &Server{
		Cfg: cfg,
		Log: cfg.Log,

		errorHandler: cfg.ErrorHandler,

		errorChan: cfg.ErrorChan,
	}

	s.server = &http.Server{
		Addr:     cfg.Address,
		Handler:  s,
		ErrorLog: s.Log.StdLogger(log.LevelError),

		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       10 * time.Second,
	}

	if cfg.TLS != nil {
		s.server.TLSConfig = &tls.Config{
			MinVersion:               tls.VersionTLS13,
			PreferServerCipherSuites: true,
		}
	}

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/", s.hNotFound)

	return s, nil
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.Cfg.Address)
	if err != nil {
		return fmt.Errorf("cannot listen on %q: %w", s.Cfg.Address, err)
	}

	s.Log.Info("listening on %s", s.Cfg.Address)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		var err error

		if s.Cfg.TLS == nil {
			err = s.server.Serve(listener)
		} else {
			certificate := s.Cfg.TLS.Certificate
			privateKey := s.Cfg.TLS.PrivateKey

			err = s.server.ServeTLS(listener, certificate, privateKey)
		}

		if err != nil {
			if err != http.ErrServerClosed {
				s.Log.Error("cannot serve: %v", err)
				s.errorChan <- fmt.Errorf("http server initialization "+
					"failed: %w", err)
			}
		}
	}()

	return nil
}

func (s *Server) Stop() {
	s.shutdown()
	s.wg.Wait()
}

func (s *Server) shutdown() {
	timeout := time.Duration(s.Cfg.ShutdownTimeout) * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		if !errors.Is(err, context.DeadlineExceeded) {
			s.Log.Error("cannot shutdown server: %v", err)
		}
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h := Handler{
		Server: s,
		Log:    s.Log.Child("", log.Data{}),

		ResponseWriter: NewResponseWriter(w),
	}

	ctx := req.Context()
	ctx = context.WithValue(ctx, contextKeyHandler, &h)
	h.Request = req.WithContext(ctx)

	h.start = time.Now()

	defer h.sendInfluxPoints()
	defer h.logRequest()

	s.mux.ServeHTTP(h.ResponseWriter, h.Request)
}

func (s *Server) Route(pathPattern, method string, routeFunc RouteFunc) {
	handlerFunc := func(w http.ResponseWriter, req *http.Request) {
		h := requestHandler(req)
		s.finalizeHandler(h, req, pathPattern, method, routeFunc)

		defer func() {
			if v := recover(); v != nil {
				msg := program.RecoverValueString(v)
				trace := program.StackTrace(2, 20, true)

				h.ReplyInternalError(500, "panic: %s\n%s", msg, trace)
			}
		}()

		routeFunc(h)
	}

	pattern := pathPattern

	if method != "" {
		pattern = method + " " + pattern
	}

	s.mux.HandleFunc(pattern, handlerFunc)

	// We usually want /foo and /foo/ to be handled the same way, so we have to
	// register both variants.

	hasSuffix := func(s string) bool {
		return strings.HasSuffix(pattern, s)
	}

	if !hasSuffix("/") && !hasSuffix("{$}") && !hasSuffix("...}") {
		s.mux.HandleFunc(pattern+"/{$}", handlerFunc)
	}
}

func (s *Server) finalizeHandler(h *Handler, req *http.Request, pathPattern, method string, routeFunc RouteFunc) {
	h.Request = req // the request may have been modified by the muxer
	h.Query = req.URL.Query()

	h.Method = method
	h.PathPattern = pathPattern
	h.RouteId = s.RouteId(method, pathPattern)

	h.ClientAddress = requestClientAddress(req)
	h.RequestId = requestId(req)

	if h.RouteId != "" {
		h.Log.Data["route"] = h.RouteId
	}

	if h.ClientAddress != "" {
		h.Log.Data["address"] = h.ClientAddress
	}

	if h.RequestId != "" {
		h.Log.Data["request_id"] = h.RequestId
	}
}

func (s *Server) RouteId(method, pathPattern string) string {
	if pathPattern == "" {
		return ""
	}

	// We want "/foo/{$}" to have the route id "/foo" and "/{$}" to have the
	// route id "/" and not "".
	pathPattern = strings.TrimSuffix(pathPattern, "/{$}")
	if pathPattern == "" {
		pathPattern = "/"
	}

	if s.Cfg.MethodLessRouteIds {
		return pathPattern
	}

	if method == "" {
		return ""
	}

	return pathPattern + " " + method
}

func DefaultErrorHandler(h *Handler, status int, code string, msg string, data ErrorData) {
	h.ReplyText(status, code+": "+msg+"\n")
}

func JSONErrorHandler(h *Handler, status int, code string, msg string, data ErrorData) {
	responseData := JSONError{
		Code:    code,
		Message: msg,
		Data:    data,
	}

	h.ReplyJSON(status, &responseData)
}

func AdaptativeErrorHandler(h *Handler, status int, code string, msg string, data ErrorData) {
	var handler ErrorHandler

	if RequestAcceptsText(h.Request) {
		handler = DefaultErrorHandler
	} else {
		handler = JSONErrorHandler
	}

	handler(h, status, code, msg, data)
}

func (s *Server) hNotFound(w http.ResponseWriter, req *http.Request) {
	h := requestHandler(req)
	s.finalizeHandler(h, req, "", req.Method, nil)

	h.ReplyError(404, "not_found", "HTTP route not found")
}

func requestHandler(req *http.Request) *Handler {
	value := req.Context().Value(contextKeyHandler)
	if value == nil {
		return nil
	}

	return value.(*Handler)
}

func requestClientAddress(req *http.Request) string {
	if v := req.Header.Get("X-Real-IP"); v != "" {
		return v
	} else if v := req.Header.Get("X-Forwarded-For"); v != "" {
		i := strings.Index(v, ", ")
		if i == -1 {
			return v
		}

		return v[:i]
	} else {
		host, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			return ""
		}

		return host
	}
}

func requestId(req *http.Request) string {
	return req.Header.Get("X-Request-Id")
}

func RequestAcceptsText(req *http.Request) bool {
	accept := req.Header.Get("Accept")
	if accept == "" {
		return false
	}

	mediaTypes := strings.Split(accept, ",")

	for _, mediaType := range mediaTypes {
		mediaType = strings.TrimSpace(mediaType)

		if strings.HasPrefix(mediaType, "text/") {
			return true
		}
	}

	return false
}
