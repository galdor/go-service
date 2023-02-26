package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/galdor/go-program"
)

type ServiceImplementation interface {
	Init(*Service) error
	Start(*Service) error
	Stop(*Service)
	Terminate(*Service)
}

type Service struct {
	Name        string
	Description string

	Implementation ServiceImplementation

	Context context.Context
}

func newService(name, description string, implementation ServiceImplementation, ctx context.Context) *Service {
	s := Service{
		Name:        name,
		Description: description,

		Implementation: implementation,

		Context: ctx,
	}

	return &s
}

func (s *Service) init() error {
	if err := s.Implementation.Init(s); err != nil {
		return err
	}

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
		fmt.Fprintf(os.Stderr, "received signal %d (%v)\n", signo, signo)

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
	// TODO

	// Service
	s := newService(name, description, implementation, ctx)

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
