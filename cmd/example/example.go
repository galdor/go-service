package main

import (
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
	Cfg ExampleCfg
	Log *log.Logger
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
	e.Log = s.Log
	return nil
}

func (e *Example) Start(s *service.Service) error {
	return nil
}

func (e *Example) Stop(s *service.Service) {
}

func (e *Example) Terminate(s *service.Service) {
}

func main() {
	service.Run("example", "a minimal example for go-service", NewExample())
}
