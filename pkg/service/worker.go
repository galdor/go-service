package service

import (
	"fmt"
	"sync"
	"time"

	"go.n16f.net/log"
	"go.n16f.net/program"
)

type WorkerFunc func(*Worker) (time.Duration, error)

type WorkerCfg struct {
	Log          *log.Logger `json:"-"`
	WorkerFunc   WorkerFunc  `json:"-"`
	Disabled     bool        `json:"disabled"`
	InitialDelay int         `json:"initial_delay"` // seconds
}

type Worker struct {
	Cfg WorkerCfg
	Log *log.Logger

	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewWorker(cfg WorkerCfg) (*Worker, error) {
	if cfg.WorkerFunc == nil {
		return nil, fmt.Errorf("missing worker function")
	}

	w := Worker{
		Cfg: cfg,
		Log: cfg.Log,

		stopChan: make(chan struct{}),
	}

	return &w, nil
}

func (w *Worker) Start() error {
	w.wg.Add(1)
	go w.main()

	return nil
}

func (w *Worker) Stop() {
	close(w.stopChan)
	w.wg.Wait()
}

func (w *Worker) main() {
	defer w.wg.Done()

	initialDelay := time.Duration(w.Cfg.InitialDelay) * time.Second

	timer := time.NewTimer(initialDelay)
	defer timer.Stop()

	for {
		select {
		case <-w.stopChan:
			return

		case <-timer.C:
			delay := 5 * time.Second

			func() {
				defer func() {
					if v := recover(); v != nil {
						msg := program.RecoverValueString(v)
						trace := program.StackTrace(0, 20, true)

						w.Log.Error("panic: %s\n%s", msg, trace)
					}
				}()

				var err error
				delay, err = w.Cfg.WorkerFunc(w)
				if err != nil {
					w.Log.Error("%v", err)
				}
			}()

			timer.Reset(delay)
		}
	}
}
