package main

import "github.com/galdor/go-service/pkg/service"

type Example struct {
}

func (s *Example) ServiceCfg() (service.ServiceCfg, error) {
	cfg := service.ServiceCfg{}
	return cfg, nil
}

func (s *Example) Init(*service.Service) error {
	return nil
}

func (s *Example) Start(*service.Service) error {
	return nil
}

func (s *Example) Stop(*service.Service) {
}

func (s *Example) Terminate(*service.Service) {
}

func NewExample() *Example {
	return &Example{}
}

func main() {
	service.Run("example", "a minimal example for go-service", NewExample())
}
