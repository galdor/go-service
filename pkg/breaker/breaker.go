package breaker

import (
	"sync"
	"time"

	"github.com/galdor/go-log"
	"github.com/galdor/go-service/pkg/utils"
)

type BreakerCfg struct {
	Log *log.Logger `json:"-"`

	ResetDelay int `json:"reset_delay"` // seconds
}

type Breaker struct {
	Cfg BreakerCfg
	Log *log.Logger

	resetDelay time.Duration

	open        bool
	openingTime *time.Time

	lock sync.Mutex
}

func NewBreaker(cfg BreakerCfg) *Breaker {
	if cfg.ResetDelay == 0 {
		cfg.ResetDelay = 10
	}

	return &Breaker{
		Cfg: cfg,
		Log: cfg.Log,

		resetDelay: time.Duration(cfg.ResetDelay) * time.Second,
	}
}

func (b *Breaker) IsClosed() bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	if !b.open {
		return true
	}

	if time.Since(*b.openingTime) >= b.resetDelay {
		b.close()
		return true
	}

	return false
}

func (b *Breaker) Open() {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.Log.Info("opening for %d seconds", b.Cfg.ResetDelay)

	b.open = true
	b.openingTime = utils.Ref(time.Now())
}

func (b *Breaker) Close() {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.close()
}

func (b *Breaker) close() {
	b.Log.Info("closing")

	b.open = false
	b.openingTime = nil
}
