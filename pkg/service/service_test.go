package service

import (
	"context"
	"errors"
	"os"
	"path"
	"runtime"
	"testing"

	"go.n16f.net/log"
	"go.n16f.net/program"
	"go.n16f.net/service/pkg/pg"
)

type TestServiceCfg struct {
	Service ServiceCfg `json:"service"`
}

type TestService struct {
	Service *Service
	Cfg     TestServiceCfg
	Log     *log.Logger

	Pg *pg.Client

	initialized bool
	started     bool
	stopped     bool
	terminated  bool

	t *testing.T
}

func NewTestService(t *testing.T) *TestService {
	return &TestService{t: t}
}

func (ts *TestService) DefaultCfg() interface{} {
	return &ts.Cfg
}

func (ts *TestService) ValidateCfg() error {
	return nil
}

func (ts *TestService) ServiceCfg() *ServiceCfg {
	return &ts.Cfg.Service
}

func (ts *TestService) Init(s *Service) error {
	ts.Service = s

	if s.Log == nil {
		ts.t.Errorf("missing logger during initialization")
	}
	ts.Log = s.Log

	if _, found := s.PgClients["main"]; !found {
		ts.t.Errorf("missing pg client during initialization")
	}
	ts.Pg = s.PgClient("main")

	ts.initialized = true
	return nil
}

func (ts *TestService) Start(s *Service) error {
	if s.Log == nil {
		ts.t.Errorf("service was not initialized during startup")
	}

	ctx := context.Background()
	var i int
	err := ts.Pg.Pool.QueryRow(ctx, "SELECT 42;").Scan(&i)
	if err != nil {
		ts.t.Errorf("cannot execute pg query: %v", err)
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

func TestMain(m *testing.M) {
	setTestDirectory()

	resetTestDatabase()

	os.Exit(m.Run())
}

func setTestDirectory() {
	// Go runs the tests of a package in the directory of this package. We
	// want tests to run at the root of the project.
	//
	// We obtain the filename of the caller which will be the path of the
	// current file. We can then get the path of the root directory of the
	// project by looking for the configuration file, and change the current
	// directory.

	cfgFileName := "test/test.yaml"

	_, filePath, _, _ := runtime.Caller(0)

	dirPath := path.Dir(filePath)
	for dirPath != "/" {
		dirPath = path.Join(dirPath, "..")

		filePath := path.Join(dirPath, cfgFileName)

		_, err := os.Stat(filePath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			program.Abort("cannot stat %q: %v", filePath, err)
		}

		break
	}

	if dirPath == "/" {
		program.Abort("%q not found", cfgFileName)
	}

	if err := os.Chdir(dirPath); err != nil {
		program.Abort("cannot change directory to %s: %v", dirPath, err)
	}
}

func resetTestDatabase() {
	clientCfg := pg.ClientCfg{
		URI: "postgres://postgres:postgres@localhost:5432/service",
	}

	client, err := pg.NewClient(clientCfg)
	if err != nil {
		program.Abort("cannot connect to the test database: %v", err)
	}
	defer client.Close()

	query := `
DROP SCHEMA public CASCADE;
CREATE SCHEMA public AUTHORIZATION postgres;
GRANT ALL ON SCHEMA public TO postgres;
`
	err = client.WithConn(func(conn pg.Conn) error {
		return pg.Exec(conn, query)
	})
	if err != nil {
		program.Abort("cannot reset test database: %v", err)
	}
}

func TestServiceLifecycle(t *testing.T) {
	readyChan := make(chan struct{})

	ts := NewTestService(t)

	go func() {
		RunTest("test-service", ts, "test/test.yaml", readyChan)
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
