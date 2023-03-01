package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/galdor/go-program"
	"github.com/galdor/go-service/pkg/log"
	"github.com/galdor/go-service/pkg/pg"
	"github.com/galdor/go-service/pkg/shttp"
	"github.com/galdor/go-service/pkg/utils"
)

type ServiceImplementation interface {
	DefaultImplementationCfg() interface{}
	ValidateImplementationCfg() error
	ServiceCfg() (*ServiceCfg, error)
	Init(*Service) error
	Start(*Service) error
	Stop(*Service)
	Terminate(*Service)
}

type ServiceCfg struct {
	name string

	Logger *log.LoggerCfg

	HTTPServers map[string]shttp.ServerCfg

	PgClients map[string]pg.ClientCfg
}

func NewServiceCfg() *ServiceCfg {
	cfg := ServiceCfg{
		HTTPServers: make(map[string]shttp.ServerCfg),

		PgClients: make(map[string]pg.ClientCfg),
	}

	return &cfg
}

func (cfg *ServiceCfg) AddHTTPServer(name string, serverCfg shttp.ServerCfg) {
	if _, found := cfg.HTTPServers[name]; found {
		utils.Panicf("duplicate http server %q", name)
	}

	cfg.HTTPServers[name] = serverCfg
}

func (cfg *ServiceCfg) AddPgClient(name string, clientCfg pg.ClientCfg) {
	if _, found := cfg.PgClients[name]; found {
		utils.Panicf("duplicate pg client %q", name)
	}

	cfg.PgClients[name] = clientCfg
}

type Service struct {
	Cfg *ServiceCfg
	Log *log.Logger

	Name           string
	Implementation ServiceImplementation

	HTTPServers map[string]*shttp.Server

	PgClients map[string]*pg.Client

	stopChan        chan struct{} // used to interrupt wait()
	errorChan       chan error    // used to signal a fatal error
	terminationChan chan struct{} // used to wait for termination in Stop()
}

func newService(cfg *ServiceCfg, implementation ServiceImplementation) *Service {
	s := Service{
		Cfg: cfg,

		Name:           cfg.name,
		Implementation: implementation,

		HTTPServers: make(map[string]*shttp.Server),

		PgClients: make(map[string]*pg.Client),

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
		s.initPgClients,
		s.initHTTPServers,
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

func (s *Service) initHTTPServers() error {
	for name, serverCfg := range s.Cfg.HTTPServers {
		serverCfg.Log = s.Log.Child("http-server", log.Data{"server": name})
		serverCfg.ErrorChan = s.errorChan

		server, err := shttp.NewServer(serverCfg)
		if err != nil {
			return fmt.Errorf("cannot create http server %q: %w", name, err)
		}

		s.HTTPServers[name] = server
	}

	return nil
}

func (s *Service) initPgClients() error {
	for name, clientCfg := range s.Cfg.PgClients {
		clientCfg.Log = s.Log.Child("pg", log.Data{"client": name})

		client, err := pg.NewClient(clientCfg)
		if err != nil {
			return fmt.Errorf("cannot create pg client %q: %w", name, err)
		}

		s.PgClients[name] = client
	}

	return nil
}

func (s *Service) start() error {
	if err := s.startHTTPServers(); err != nil {
		return err
	}

	if err := s.Implementation.Start(s); err != nil {
		return err
	}

	s.Log.Info("started")

	return nil
}

func (s *Service) startHTTPServers() error {
	for name, s := range s.HTTPServers {
		if err := s.Start(); err != nil {
			return fmt.Errorf("cannot start http server %q: %w", name, err)
		}
	}

	return nil
}

func (s *Service) wait() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case signo := <-sigChan:
		// Cosmetic fix to avoid having "^C" displayed before the next log
		// line in shells which print interrupting characters.
		fmt.Fprintln(os.Stderr)
		s.Log.Info("received signal %d (%v)", signo, signo)

	case <-s.stopChan:

	case err := <-s.errorChan:
		s.Log.Error("service error: %v", err)
		os.Exit(1)
	}
}

func (s *Service) stop() error {
	s.Log.Info("stopping")

	s.Implementation.Stop(s)

	for _, s := range s.HTTPServers {
		s.Stop()
	}

	s.stopPgClients()

	s.Log.Info("stopped")

	return nil
}

func (s *Service) stopPgClients() {
	for _, client := range s.PgClients {
		client.Close()
	}
}

func (s *Service) terminate() error {
	s.Implementation.Terminate(s)

	close(s.stopChan)
	close(s.errorChan)
	close(s.terminationChan)

	return nil
}

func Run(name, description string, implementation ServiceImplementation) {
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
	s := newService(serviceCfg, implementation)

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
	// Configuration
	implementationCfg := implementation.DefaultImplementationCfg()

	if cfgPath != "" {
		if err := LoadCfg(cfgPath, implementationCfg); err != nil {
			utils.Abort("cannot load configuration: %v", err)
		}

		if err := implementation.ValidateImplementationCfg(); err != nil {
			utils.Abort("invalid configuration: %v", err)
		}
	}

	serviceCfg, err := implementation.ServiceCfg()
	if err != nil {
		utils.Abort("invalid configuration: %v", err)
	}

	serviceCfg.name = name

	// Service
	s := newService(serviceCfg, implementation)

	if err := s.init(); err != nil {
		utils.Abort("cannot initialize service: %v", err)
	}

	if err := s.start(); err != nil {
		utils.Abort("cannot start service: %v", err)
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

func (s *Service) HTTPServer(name string) *shttp.Server {
	server, found := s.HTTPServers[name]
	if !found {
		utils.Panicf("unknown http server %q", name)
	}

	return server
}

func (s *Service) PgClient(name string) *pg.Client {
	c, found := s.PgClients[name]
	if !found {
		utils.Panicf("unknown pg client %q", name)
	}

	return c
}
