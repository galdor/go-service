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

	Context context.Context
}

func newService(cfg ServiceCfg, implementation ServiceImplementation, ctx context.Context) *Service {
	s := Service{
		Cfg: cfg,

		Name:           cfg.name,
		Implementation: implementation,

		Context: ctx,
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
		fmt.Println()
		s.Log.Info("received signal %d (%v)", signo, signo)

	case <-s.Context.Done():
	}
}

func (s *Service) stop() error {
	s.Implementation.Stop(s)

	return nil
}

func (s *Service) terminate() error {
	s.Implementation.Terminate(s)

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
