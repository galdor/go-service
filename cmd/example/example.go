package main

import (
	"fmt"

	"github.com/galdor/go-service/pkg/log"
	"github.com/galdor/go-service/pkg/pg"
	"github.com/galdor/go-service/pkg/service"
	"github.com/galdor/go-service/pkg/shttp"
)

type ExampleCfg struct {
	Logger        *log.LoggerCfg  `json:"logger"`
	APIHTTPServer shttp.ServerCfg `json:"apiHTTPServer"`
	Pg            pg.ClientCfg    `json:"pg"`
}

type Example struct {
	Cfg     ExampleCfg
	Service *service.Service
	Log     *log.Logger
}

func NewExample() *Example {
	return &Example{}
}

func (e *Example) DefaultImplementationCfg() interface{} {
	return &e.Cfg
}

func (e *Example) ValidateImplementationCfg() error {
	return nil
}

func (e *Example) ServiceCfg() (*service.ServiceCfg, error) {
	cfg := service.NewServiceCfg()

	cfg.Logger = e.Cfg.Logger

	cfg.AddHTTPServer("api", e.Cfg.APIHTTPServer)

	cfg.AddPgClient("main", e.Cfg.Pg)

	return cfg, nil
}

func (e *Example) Init(s *service.Service) error {
	e.Service = s
	e.Log = s.Log

	e.initAPIHTTPRoutes()

	return nil
}

func (e *Example) initAPIHTTPRoutes() {
	s := e.Service.HTTPServer("api")

	s.Route("/hello/:name", "GET", e.hAPIHelloGET)
}

func (e *Example) Start(s *service.Service) error {
	return nil
}

func (e *Example) Stop(s *service.Service) {
}

func (e *Example) Terminate(s *service.Service) {
}

func (e *Example) hAPIHelloGET(h *shttp.Handler) {
	name := h.PathVariable("name")
	h.ReplyText(200, fmt.Sprintf("Hello %s!\n", name))
}

func main() {
	service.Run("example", "a minimal example for go-service", NewExample())
}
