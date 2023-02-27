package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/galdor/go-program"
	"github.com/galdor/go-service/pkg/log"
)

type ServiceImplementation interface {
	DefaultImplementationCfg() interface{}
	ValidateImplementationCfg() error
	ServiceCfg() (ServiceCfg, error)
	Init(*Service) error
	Start(*Service) error
	Stop(*Service)
	Terminate(*Service)
}

type ServiceCfg struct {
	name string

	Logger *log.LoggerCfg
}

type Service struct {
	Cfg ServiceCfg
	Log *log.Logger

	Name           string
	Implementation ServiceImplementation

	stopChan        chan struct{} // used to interrupt wait()
	errorChan       chan error    // used to signal a fatal error
	terminationChan chan struct{} // used to wait for termination in Stop()
}

func newService(cfg ServiceCfg, implementation ServiceImplementation, ctx context.Context) *Service {
	s := Service{
		Cfg: cfg,

		Name:           cfg.name,
		Implementation: implementation,

		stopChan:        make(chan struct{}, 1),
		errorChan:       make(chan error),
		terminationChan: make(chan struct{}),
	}

	return &s
}

func (s *Service) init() error {
	s.Log = log.DefaultLogger(s.Name)

	initFuncs := []func() error{
		s.initLogger,
	}

	for _, initFunc := range initFuncs {
		if err := initFunc(); err != nil {
			return err
		}
	}

	if err := s.Implementation.Init(s); err != nil {
		return err
	}

	return nil
}

func (s *Service) initLogger() error {
	if s.Cfg.Logger == nil {
		return nil
	}

	logger, err := log.NewLogger(s.Name, *s.Cfg.Logger)
	if err != nil {
		return fmt.Errorf("invalid logger configuration: %w", err)
	}

	s.Log = logger

	return nil
}

func (s *Service) start() error {
	if err := s.Implementation.Start(s); err != nil {
		return err
	}

	return nil
}

func (s *Service) wait() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case signo := <-sigChan:
		s.Log.Info("received signal %d (%v)", signo, signo)

	case <-s.stopChan:

	case err := <-s.errorChan:
		s.Log.Error("service error: %v", err)
		os.Exit(1)
	}
}

func (s *Service) stop() error {
	s.Implementation.Stop(s)

	return nil
}

func (s *Service) terminate() error {
	s.Implementation.Terminate(s)

	close(s.stopChan)
	close(s.errorChan)
	close(s.terminationChan)

	return nil
}

func Run(name, description string, implementation ServiceImplementation) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Program
	p := program.NewProgram(name, description)

	p.AddOption("c", "cfg-file", "path", "",
		"the path of the configuration file")
	p.AddFlag("", "validate-cfg",
		"validate the configuration and exit")

	p.ParseCommandLine()

	// Configuration
	implementationCfg := implementation.DefaultImplementationCfg()

	if p.IsOptionSet("cfg-file") {
		cfgPath := p.OptionValue("cfg-file")

		p.Info("loading configuration from %q", cfgPath)

		if err := LoadCfg(cfgPath, implementationCfg); err != nil {
			p.Fatal("cannot load configuration: %v", err)
		}

		if err := implementation.ValidateImplementationCfg(); err != nil {
			p.Fatal("invalid configuration: %v", err)
		}
	}

	serviceCfg, err := implementation.ServiceCfg()
	if err != nil {
		p.Fatal("invalid configuration: %v", err)
	}

	serviceCfg.name = name

	if p.IsOptionSet("validate-cfg") {
		p.Info("configuration validated successfully")
		return
	}

	// Service
	s := newService(serviceCfg, implementation, ctx)

	if err := s.init(); err != nil {
		p.Fatal("cannot initialize service: %v", err)
	}

	if err := s.start(); err != nil {
		p.Fatal("cannot start service: %v", err)
	}

	s.wait()

	s.stop()

	s.terminate()
}

func RunTest(name string, implementation ServiceImplementation, cfgPath string, readyChan chan<- struct{}) {
	abort := func(format string, args ...interface{}) {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
		os.Exit(1)
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Configuration
	implementationCfg := implementation.DefaultImplementationCfg()

	if cfgPath != "" {
		if err := LoadCfg(cfgPath, implementationCfg); err != nil {
			abort("cannot load configuration: %v", err)
		}

		if err := implementation.ValidateImplementationCfg(); err != nil {
			abort("invalid configuration: %v", err)
		}
	}

	serviceCfg, err := implementation.ServiceCfg()
	if err != nil {
		abort("invalid configuration: %v", err)
	}

	serviceCfg.name = name

	// Service
	s := newService(serviceCfg, implementation, ctx)

	if err := s.init(); err != nil {
		abort("cannot initialize service: %v", err)
	}

	if err := s.start(); err != nil {
		abort("cannot start service: %v", err)
	}

	close(readyChan)

	s.wait()

	s.stop()

	s.terminate()
}

func (s *Service) Stop() {
	s.stopChan <- struct{}{}

	select {
	case <-s.terminationChan:
	}
}
