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

	wakeupChan chan struct{}
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

func NewWorker(cfg WorkerCfg) (*Worker, error) {
	if cfg.WorkerFunc == nil {
		return nil, fmt.Errorf("missing worker function")
	}

	w := Worker{
		Cfg: cfg,
		Log: cfg.Log,

		wakeupChan: make(chan struct{}),
		stopChan:   make(chan struct{}),
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

	close(w.wakeupChan)
}

func (w *Worker) main() {
	defer w.wg.Done()

	initialDelay := time.Duration(w.Cfg.InitialDelay) * time.Second

	timer := time.NewTimer(initialDelay)
	defer timer.Stop()

	callFunc := func() {
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

	for {
		select {
		case <-w.stopChan:
			return

		case <-w.wakeupChan:
			callFunc()

		case <-timer.C:
			callFunc()
		}
	}
}

func (w *Worker) WakeUp() {
	// Note how we do not wait for the worker to read the channel. If it is not
	// currently sleeping (i.e. it is executing the worker function), there is
	// no point in waiting for the worker function to return.

	select {
	case w.wakeupChan <- struct{}{}:
	default:
	}
}
