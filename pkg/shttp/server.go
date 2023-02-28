package shttp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/galdor/go-service/pkg/log"
)

type ServerCfg struct {
	Log       *log.Logger  `json:"-"`
	ErrorChan chan<- error `json:"-"`

	Address string `json:"address"`

	TLS *TLSServerCfg `json:"tls,omitempty"`
}

type TLSServerCfg struct {
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"privateKey"`
}

type Server struct {
	Cfg ServerCfg
	Log *log.Logger

	server *http.Server

	errorChan chan<- error
	wg        sync.WaitGroup
}

func NewServer(cfg ServerCfg) (*Server, error) {
	if cfg.Log == nil {
		cfg.Log = log.DefaultLogger("http-server")
	}

	if cfg.ErrorChan == nil {
		return nil, fmt.Errorf("missing error channel")
	}

	if cfg.Address == "" {
		cfg.Address = "localhost:8080"
	}

	s := &Server{
		Cfg: cfg,
		Log: cfg.Log,

		errorChan: cfg.ErrorChan,
	}

	s.server = &http.Server{
		Addr:     cfg.Address,
		Handler:  s,
		ErrorLog: s.Log.StdLogger(log.LevelError),
	}

	if cfg.TLS != nil {
		s.server.TLSConfig = &tls.Config{
			MinVersion:               tls.VersionTLS13,
			PreferServerCipherSuites: true,
		}
	}

	return s, nil
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.Cfg.Address)
	if err != nil {
		return fmt.Errorf("cannot listen on %q: %w", s.Cfg.Address, err)
	}

	s.Log.Info("listening on %q", s.Cfg.Address)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		var err error

		if s.Cfg.TLS == nil {
			err = s.server.Serve(listener)
		} else {
			certificate := s.Cfg.TLS.Certificate
			privateKey := s.Cfg.TLS.PrivateKey

			err = s.server.ServeTLS(listener, certificate, privateKey)
		}

		if err != nil {
			if err != http.ErrServerClosed {
				s.Log.Error("cannot serve: %v", err)
				s.errorChan <- fmt.Errorf("http server initialization "+
					"failed: %w", err)
			}
		}
	}()

	return nil
}

func (s *Server) Stop() {
	s.shutdown()
	s.wg.Wait()
}

func (s *Server) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		s.Log.Error("cannot shutdown server: %v", err)
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// TODO
	w.WriteHeader(501)
	fmt.Fprintf(w, "request handling not implemented yet")
}
