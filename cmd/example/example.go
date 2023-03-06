package main

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/galdor/go-service/pkg/log"
	"github.com/galdor/go-service/pkg/service"
	"github.com/galdor/go-service/pkg/shttp"
)

type ExampleCfg struct {
	Service service.ServiceCfg `json:"service"`
}

type Example struct {
	Cfg     ExampleCfg
	Service *service.Service
	Log     *log.Logger
}

func NewExample() *Example {
	return &Example{}
}

func (e *Example) DefaultCfg() interface{} {
	return &e.Cfg
}

func (e *Example) ValidateCfg() error {
	return nil
}

func (e *Example) ServiceCfg() *service.ServiceCfg {
	return &e.Cfg.Service
}

func (e *Example) Init(s *service.Service) error {
	e.Service = s
	e.Log = s.Log

	e.initAPIHTTPRoutes()

	return nil
}

func (e *Example) initAPIHTTPRoutes() {
	s := e.Service.HTTPServer("api")

	s.Route("/ping", "GET", e.hAPIPingGET)
	s.Route("/hello/:name", "GET", e.hAPIHelloNameGET)
	s.Route("/error/internal", "GET", e.hErrorInternalGET)
	s.Route("/error/panic", "GET", e.hErrorPanicGET)
}

func (e *Example) Start(s *service.Service) error {
	return nil
}

func (e *Example) Stop(s *service.Service) {
}

func (e *Example) Terminate(s *service.Service) {
}

func (e *Example) hAPIPingGET(h *shttp.Handler) {
	h.ReplyText(200, "pong")
}

func (e *Example) hAPIHelloNameGET(h *shttp.Handler) {
	n := 1

	if h.HasQueryParameter("n") {
		i64, err := strconv.ParseInt(h.QueryParameter("n"), 10, 64)
		if err != nil || i64 < 1 || i64 > 10 {
			msg := fmt.Sprintf("invalid value for query parameter %q\n", "n")
			h.ReplyText(400, msg)
			return
		}

		n = int(i64)
	}

	name := h.PathVariable("name")

	var response bytes.Buffer
	for i := 0; i < n; i++ {
		fmt.Fprintf(&response, "Hello %s!\n", name)
	}

	h.ReplyText(200, response.String())
}

func (e *Example) hErrorInternalGET(h *shttp.Handler) {
	h.ReplyInternalError(500, "test error")
}

func (e *Example) hErrorPanicGET(h *shttp.Handler) {
	panic("test error")
}

func main() {
	service.Run("example", "a minimal example for go-service", NewExample())
}
