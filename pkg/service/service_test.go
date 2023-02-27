package service

import (
	"testing"

	"github.com/galdor/go-service/pkg/log"
)

type TestServiceCfg struct {
	Logger *log.LoggerCfg `json:"logger"`
}

type TestService struct {
	Service *Service
	Cfg     TestServiceCfg
	Log     *log.Logger

	initialized bool
	started     bool
	stopped     bool
	terminated  bool

	t *testing.T
}

func NewTestService(t *testing.T) *TestService {
	return &TestService{t: t}
}

func (ts *TestService) DefaultImplementationCfg() interface{} {
	return &ts.Cfg
}

func (ts *TestService) ValidateImplementationCfg() error {
	return nil
}

func (ts *TestService) ServiceCfg() (*ServiceCfg, error) {
	cfg := NewServiceCfg()

	cfg.Logger = ts.Cfg.Logger

	return cfg, nil
}

func (ts *TestService) Init(s *Service) error {
	if s.Log == nil {
		ts.t.Errorf("service logger is nil during initialization")
	}

	ts.Service = s
	ts.Log = s.Log

	ts.initialized = true
	return nil
}

func (ts *TestService) Start(s *Service) error {
	if s.Log == nil {
		ts.t.Errorf("service was not initialized during startup")
	}

	ts.started = true
	return nil
}

func (ts *TestService) Stop(s *Service) {
	if s.Log == nil {
		ts.t.Errorf("service was not started before being stopped")
	}

	ts.stopped = true
}

func (ts *TestService) Terminate(s *Service) {
	if s.Log == nil {
		ts.t.Errorf("service was not stopped before termination")
	}

	ts.terminated = true
}

func TestServiceLifecycle(t *testing.T) {
	readyChan := make(chan struct{})

	ts := NewTestService(t)

	go func() {
		RunTest("test-service", ts, "", readyChan)
	}()

	select {
	case <-readyChan:
		ts.Service.Stop()
	}

	if !ts.initialized {
		t.Errorf("service was not initialized")
	}

	if !ts.started {
		t.Errorf("service was not started")
	}

	if !ts.stopped {
		t.Errorf("service was not stopped")
	}

	if !ts.terminated {
		t.Errorf("service was not terminated")
	}
}
