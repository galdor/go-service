package service

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"os"
	"os/signal"
	"path"
	"syscall"
	texttemplate "text/template"

	"go.n16f.net/ejson"
	"go.n16f.net/log"
	"go.n16f.net/program"
	"go.n16f.net/service/pkg/influx"
	"go.n16f.net/service/pkg/pg"
	"go.n16f.net/service/pkg/shttp"
)

type ServiceImplementation interface {
	DefaultCfg() interface{}
	ValidateCfg() error
	ServiceCfg() *ServiceCfg
	Init(*Service) error
	Start(*Service) error
	Stop(*Service)
	Terminate(*Service)
}

type ServiceImplementationWithInitProgram interface {
	ServiceImplementation

	InitProgram(*program.Program)
}

type ServiceCfg struct {
	name string

	Logger *log.LoggerCfg `json:"logger"`

	DataDirectory string `json:"data_directory"`

	Influx *influx.ClientCfg `json:"influx"`

	PgClients map[string]*pg.ClientCfg `json:"pg_clients"`

	HTTPClients map[string]*shttp.ClientCfg `json:"http_clients"`
	HTTPServers map[string]*shttp.ServerCfg `json:"http_servers"`

	ServiceAPI *ServiceAPICfg `json:"service_api"`

	Workers map[string]*WorkerCfg `json:"workers"`

	DisableTemplateLoading bool                   `json:"-"`
	TemplateFuncMap        map[string]interface{} `json:"-"`
}

type Service struct {
	Cfg *ServiceCfg
	Log *log.Logger

	Name           string
	Implementation ServiceImplementation

	Program *program.Program

	Hostname string

	Influx *influx.Client

	PgClients map[string]*pg.Client

	HTTPClients map[string]*shttp.Client
	HTTPServers map[string]*shttp.Server

	ServiceAPI *ServiceAPI

	Workers map[string]*Worker

	TextTemplate *texttemplate.Template
	HTMLTemplate *htmltemplate.Template

	stopChan        chan struct{} // used to interrupt wait()
	errorChan       chan error    // used to signal a fatal error
	terminationChan chan struct{} // used to wait for termination in Stop()
}

func (cfg *ServiceCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckOptionalObject("logger", cfg.Logger)

	v.CheckStringNotEmpty("data_directory", cfg.DataDirectory)

	v.CheckOptionalObject("influx", cfg.Influx)

	v.Push("pg_clients")
	for name, clientCfg := range cfg.PgClients {
		v.CheckObject(name, clientCfg)
	}
	v.Pop()

	v.Push("http_clients")
	for name, clientCfg := range cfg.HTTPClients {
		v.CheckObject(name, clientCfg)
	}
	v.Pop()

	v.Push("http_servers")
	for name, serverCfg := range cfg.HTTPServers {
		v.CheckObject(name, serverCfg)
	}
	v.Pop()

	v.CheckOptionalObject("service_api", cfg.ServiceAPI)
}

func newService(cfg *ServiceCfg, implementation ServiceImplementation) *Service {
	s := Service{
		Cfg: cfg,

		Name:           cfg.name,
		Implementation: implementation,

		HTTPClients: make(map[string]*shttp.Client),
		HTTPServers: make(map[string]*shttp.Server),

		PgClients: make(map[string]*pg.Client),

		Workers: make(map[string]*Worker),

		stopChan:        make(chan struct{}, 1),
		errorChan:       make(chan error),
		terminationChan: make(chan struct{}),
	}

	return &s
}

func (s *Service) init() error {
	s.Log = log.DefaultLogger(s.Name)

	initFuncs := []func() error{
		s.initHostname,
		s.initLogger,
		s.initTemplates,
		s.initInflux,
		s.initPgClients,
		s.initHTTPServers,
		s.initHTTPClients,
		s.initServiceAPI,
		s.initWorkers,
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

func (s *Service) initHostname() error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("cannot obtain hostname: %w", err)
	}

	s.Hostname = hostname

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

func (s *Service) initTemplates() error {
	if s.Cfg.DisableTemplateLoading {
		return nil
	}

	dirPath := path.Join(s.Cfg.DataDirectory, "templates")

	textTemplate, htmlTemplate, err := LoadTemplates(dirPath,
		s.Cfg.TemplateFuncMap)
	if err != nil {
		return fmt.Errorf("cannot load templates: %w", err)
	}

	s.TextTemplate = textTemplate
	s.HTMLTemplate = htmlTemplate

	return nil
}

func (s *Service) initInflux() error {
	if s.Cfg.Influx == nil {
		return nil
	}

	httpClientCfg := shttp.ClientCfg{
		LogRequests: s.Cfg.Influx.LogRequests,
	}

	httpClient, err := shttp.NewClient(httpClientCfg)
	if err != nil {
		return fmt.Errorf("cannot create Influx HTTP client: %w", err)
	}

	cfg := *s.Cfg.Influx

	cfg.Log = s.Log.Child("influx", log.Data{})
	cfg.HTTPClient = httpClient.Client
	cfg.Hostname = s.Hostname

	client, err := influx.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("cannot create Influx client: %w", err)
	}

	s.Influx = client

	return nil
}

func (s *Service) initHTTPServers() error {
	for name, serverCfg := range s.Cfg.HTTPServers {
		serverCfg.Log = s.Log.Child("http_server", log.Data{"server": name})
		serverCfg.ErrorChan = s.ErrorChan()
		serverCfg.InfluxClient = s.Influx
		serverCfg.Name = name
		serverCfg.DataDirectory = s.Cfg.DataDirectory

		server, err := shttp.NewServer(*serverCfg)
		if err != nil {
			return fmt.Errorf("cannot create HTTP server %q: %w", name, err)
		}

		s.HTTPServers[name] = server
	}

	return nil
}

func (s *Service) initHTTPClients() error {
	for name, clientCfg := range s.Cfg.HTTPClients {
		clientCfg.Log = s.Log.Child("http_client", log.Data{"client": name})

		client, err := shttp.NewClient(*clientCfg)
		if err != nil {
			return fmt.Errorf("cannot create HTTP client %q: %w", name, err)
		}

		s.HTTPClients[name] = client
	}

	return nil
}

func (s *Service) initServiceAPI() error {
	if s.Cfg.ServiceAPI == nil {
		return nil
	}

	apiCfg := *s.Cfg.ServiceAPI

	apiCfg.Log = s.Log.Child("service_api", log.Data{})
	apiCfg.Service = s

	s.ServiceAPI = NewServiceAPI(apiCfg)

	return nil
}

func (s *Service) initWorkers() error {
	for name, workerCfg := range s.Cfg.Workers {
		if workerCfg.Disabled {
			continue
		}

		workerCfg.Log = s.Log.Child("worker", log.Data{"worker": name})

		worker, err := NewWorker(*workerCfg)
		if err != nil {
			return fmt.Errorf("cannot create worker %q: %w", name, err)
		}

		s.Workers[name] = worker
	}

	return nil
}

func (s *Service) initPgClients() error {
	defaultSchemaDirectory := path.Join(s.Cfg.DataDirectory, "pg", "schemas")
	for name, clientCfg := range s.Cfg.PgClients {
		clientCfg.Log = s.Log.Child("pg", log.Data{"client": name})
		clientCfg.InfluxClient = s.Influx
		clientCfg.Name = name

		if clientCfg.SchemaDirectory == "" {
			clientCfg.SchemaDirectory = defaultSchemaDirectory
		}

		client, err := pg.NewClient(*clientCfg)
		if err != nil {
			return fmt.Errorf("cannot create pg client %q: %w", name, err)
		}

		s.PgClients[name] = client
	}

	return nil
}

func (s *Service) start() error {
	s.Log.Debug(1, "starting")

	// The implementation may use the Influx client so we start it first.
	if s.Influx != nil {
		s.Influx.Start()
	}

	if err := s.Implementation.Start(s); err != nil {
		return err
	}

	// HTTP server handlers may use systems started by the implementation. So
	// do workers. So they all start after the implemention.

	if err := s.startHTTPServers(); err != nil {
		return err
	}

	if s.ServiceAPI != nil {
		if err := s.ServiceAPI.Start(); err != nil {
			return err
		}
	}

	if err := s.startWorkers(); err != nil {
		return err
	}

	s.Log.Debug(1, "started")

	return nil
}

func (s *Service) startHTTPServers() error {
	for name, s := range s.HTTPServers {
		if err := s.Start(); err != nil {
			return fmt.Errorf("cannot start HTTP server %q: %w", name, err)
		}
	}

	return nil
}

func (s *Service) startWorkers() error {
	for name, w := range s.Workers {
		if err := w.Start(); err != nil {
			return fmt.Errorf("cannot start worker %q: %w", name, err)
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
	s.Log.Debug(1, "stopping")

	s.stopWorkers()

	if s.ServiceAPI != nil {
		s.ServiceAPI.Stop()
	}

	s.stopHTTPServers()

	s.Implementation.Stop(s)

	s.stopHTTPClients()

	s.stopPgClients()

	if s.Influx != nil {
		s.Influx.Stop()
	}

	s.Log.Debug(1, "stopped")

	return nil
}

func (s *Service) stopWorkers() {
	for _, w := range s.Workers {
		w.Stop()
	}
}

func (s *Service) stopHTTPServers() {
	for _, server := range s.HTTPServers {
		server.Stop()
	}
}

func (s *Service) stopPgClients() {
	for _, client := range s.PgClients {
		client.Close()
	}
}

func (s *Service) stopHTTPClients() {
	for _, client := range s.HTTPClients {
		client.CloseConnections()
	}
}

func (s *Service) terminate() error {
	s.Implementation.Terminate(s)

	if s.Influx != nil {
		s.Influx.Terminate()
	}

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

	if i2, ok := implementation.(ServiceImplementationWithInitProgram); ok {
		i2.InitProgram(p)
	}

	p.ParseCommandLine()

	// Configuration
	cfg := implementation.DefaultCfg()

	if p.IsOptionSet("cfg-file") {
		cfgPath := p.OptionValue("cfg-file")

		p.Info("loading configuration from %q", cfgPath)

		templateData := map[string]interface{}{
			"Program": p,
		}

		if err := LoadCfg(cfgPath, templateData, cfg); err != nil {
			p.Fatal("cannot load configuration: %v", err)
		}

		if err := implementation.ValidateCfg(); err != nil {
			p.Fatal("invalid configuration: %v", err)
		}
	}

	serviceCfg := implementation.ServiceCfg()

	serviceCfg.name = name

	if p.IsOptionSet("validate-cfg") {
		p.Info("configuration validated successfully")
		return
	}

	// The --debug command line option overrides what is in the configuration
	// file.
	if p.IsOptionSet("debug") {
		serviceCfg.Logger.DebugLevel = p.DebugLevel
	}

	// Service
	//
	// We setup recovery early to catch potential initialization panic.
	s := newService(serviceCfg, implementation)
	s.Program = p

	defer func() {
		if v := recover(); v != nil {
			msg := program.RecoverValueString(v)
			trace := program.StackTrace(0, 20, true)

			if s.Log == nil {
				p.Error("panic: %s\n%s", msg, trace)
			} else {
				s.Log.Error("panic: %s\n%s", msg, trace)
				os.Exit(1)
			}
		}
	}()

	if err := s.init(); err != nil {
		// We want to use the service logger as much as possible. It is
		// initialized first in (*Service).init so most of the time we should
		// be able to use it.

		if s.Log == nil {
			p.Fatal("cannot initialize service: %v", err)
		} else {
			s.Log.Error("cannot initialize service: %v", err)
			os.Exit(1)
		}
	}

	if err := s.start(); err != nil {
		s.Log.Error("cannot start service: %v", err)
		os.Exit(1)
	}

	s.wait()

	s.stop()

	s.terminate()
}

func RunTest(name string, implementation ServiceImplementation, cfgPath string, readyChan chan<- struct{}) {
	// Configuration
	cfg := implementation.DefaultCfg()

	if cfgPath != "" {
		if err := LoadCfg(cfgPath, nil, cfg); err != nil {
			program.Abort("cannot load configuration: %v", err)
		}

		if err := implementation.ValidateCfg(); err != nil {
			program.Abort("invalid configuration: %v", err)
		}
	}

	serviceCfg := implementation.ServiceCfg()

	serviceCfg.name = name

	// Service
	s := newService(serviceCfg, implementation)

	if err := s.init(); err != nil {
		program.Abort("cannot initialize service: %v", err)
	}

	if err := s.start(); err != nil {
		program.Abort("cannot start service: %v", err)
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

func (s *Service) ErrorChan() chan<- error {
	return s.errorChan
}

func (s *Service) HTTPClient(name string) *shttp.Client {
	client, found := s.HTTPClients[name]
	if !found {
		program.Panic("unknown http client %q", name)
	}

	return client
}

func (s *Service) HTTPServer(name string) *shttp.Server {
	server, found := s.HTTPServers[name]
	if !found {
		program.Panic("unknown http server %q", name)
	}

	return server
}

func (s *Service) PgClient(name string) *pg.Client {
	client, found := s.PgClients[name]
	if !found {
		program.Panic("unknown pg client %q", name)
	}

	return client
}

func (s *Service) Worker(name string) *Worker {
	worker, found := s.Workers[name]
	if !found {
		program.Panic("unknown worker %q", name)
	}

	return worker
}

func (s *Service) AddTemplateFunctions(functions map[string]interface{}) {
	s.TextTemplate.Funcs(functions)
	s.HTMLTemplate.Funcs(functions)
}

func (s *Service) RenderTextTemplate(name string, data interface{}) ([]byte, error) {
	var buf bytes.Buffer

	if err := s.TextTemplate.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *Service) RenderHTMLTemplate(name string, data interface{}) ([]byte, error) {
	var buf bytes.Buffer

	if err := s.HTMLTemplate.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
