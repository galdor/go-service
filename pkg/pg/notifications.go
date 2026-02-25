package pg

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"go.n16f.net/log"
)

type NotificationSubscription struct {
	C        chan string
	listener *Listener
}

func (s *NotificationSubscription) Cancel() {
	s.listener.subscriptionMutex.Lock()
	defer s.listener.subscriptionMutex.Unlock()

	s.listener.subscriptions = slices.DeleteFunc(s.listener.subscriptions,
		func(sub *NotificationSubscription) bool {
			return sub == s
		})

	if s.C != nil {
		close(s.C)
		s.C = nil
	}
}

type Listener struct {
	Log *log.Logger

	connConfig *pgx.ConnConfig
	channel    string

	subscriptionMutex sync.Mutex
	subscriptions     []*NotificationSubscription

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func (l *Listener) Close() {
	l.cancel()
	l.wg.Wait()

	l.subscriptionMutex.Lock()
	defer l.subscriptionMutex.Unlock()

	for _, sub := range l.subscriptions {
		close(sub.C)
		sub.C = nil
	}
	l.subscriptions = nil
}

func (l *Listener) main() {
	var conn *pgx.Conn

	closeConn := func() {
		if conn != nil {
			ctx, cancel := context.WithTimeout(context.Background(),
				time.Second)
			defer cancel()

			conn.Close(ctx)
			conn = nil
		}
	}

	defer closeConn()

	delay := 0
	maxDelay := 60

	timer := time.NewTimer(0)
	defer timer.Stop()

	increaseDelay := func() {
		if delay == 0 {
			delay = 1
		} else {
			delay = min(delay*2, maxDelay)
		}

		timer.Reset(time.Duration(delay) * time.Second)

	}

	resetDelay := func() {
		delay = 0
	}

loop:
	for {
		select {
		case <-timer.C:

		case <-l.ctx.Done():
			return
		}

		var err error
		conn, err = pgx.ConnectConfig(l.ctx, l.connConfig)
		if err != nil {
			l.Log.Error("cannot create connection: %v", err)
			increaseDelay()
			continue
		}

		_, err = conn.Exec(l.ctx, "LISTEN "+QuoteIdentifier(l.channel))
		if err != nil {
			l.Log.Error("cannot listen to channel: %v", err)
			closeConn()
			increaseDelay()
			continue
		}

		resetDelay()

		for {
			notification, err := conn.WaitForNotification(l.ctx)
			if err != nil {
				select {
				case <-l.ctx.Done():
				default:
					l.Log.Error("cannot read notification: %v", err)
				}

				closeConn()
				increaseDelay()
				continue loop
			}

			func() {
				l.subscriptionMutex.Lock()
				defer l.subscriptionMutex.Unlock()

				for _, sub := range l.subscriptions {
					select {
					case sub.C <- notification.Payload:

					case <-l.ctx.Done():
						return

					default:
						// Do not block all subscribers is one is particularly
						// slow.
					}
				}
			}()
		}
	}
}

func (c *Client) ensureListener(channel string) (*Listener, error) {
	c.listenerMutex.Lock()
	defer c.listenerMutex.Unlock()

	if existingListener, found := c.listeners[channel]; found {
		return existingListener, nil
	}

	listenerCtx, listenerCancel := context.WithCancel(context.Background())

	listener := Listener{
		Log: c.Log.Child("listener", log.Data{"channel": channel}),

		connConfig: c.Pool.Config().ConnConfig,
		channel:    channel,

		ctx:    listenerCtx,
		cancel: listenerCancel,
	}

	c.listeners[channel] = &listener

	listener.wg.Go(listener.main)

	return &listener, nil
}

func (c *Client) Listen(channel string) (*NotificationSubscription, error) {
	listener, err := c.ensureListener(channel)
	if err != nil {
		return nil, err
	}

	sub := NotificationSubscription{
		C: make(chan string, 1),

		listener: listener,
	}

	listener.subscriptionMutex.Lock()
	listener.subscriptions = append(listener.subscriptions, &sub)
	listener.subscriptionMutex.Unlock()

	return &sub, nil
}
