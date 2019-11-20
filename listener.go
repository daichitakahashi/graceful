package graceful

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Listener :
type Listener interface {
	net.Listener
	Shutdown() error // equivalent to Close
	GracefulShutdown(time.Duration) error
	StopAccept() (func(), error)
}

// Upgrade :
func Upgrade(ln net.Listener) Listener {
	listener := &gracefulListener{
		Listener: ln,
		w:        &waiter{},
	}
	listener.w.Reset()
	return listener
}

type gracefulListener struct {
	net.Listener
	w       *waiter
	once    sync.Once
	stop    uint32
	suspend chan struct{}
}

func (ln *gracefulListener) Accept() (net.Conn, error) {
	if atomic.LoadUint32(&ln.stop) != 0 {
		<-ln.suspend
	}
	c, err := ln.Listener.Accept()
	if err != nil {
		return nil, err
	}
	ln.w.Add()
	return &conn{
		Conn:   c,
		parent: ln,
	}, nil
}

func (ln *gracefulListener) Close() error { return ln.Listener.Close() }

func (ln *gracefulListener) closeConn() { ln.w.Done() }

func (ln *gracefulListener) Shutdown() error { return ln.Close() }

var (
	// ErrAlreadyStopped :
	ErrAlreadyStopped = errors.New("already stopped")

	// ErrTimeout :
	ErrTimeout = errors.New("timeout: some Conn remain alive")
)

func (ln *gracefulListener) GracefulShutdown(timeout time.Duration) error {
	restart, err := ln.StopAccept()
	if err != nil {
		return err
	}
	defer restart()

	done := make(chan struct{})
	go func() {
		ln.w.Wait()
		close(done)
	}()

	if timeout > 0 {
		select {
		case <-time.After(timeout):
			ln.w.CancelWait() // need??
			return ErrTimeout
		case <-done:
		}
	} else {
		<-done
	}
	return ln.Listener.Close()
}

func (ln *gracefulListener) StopAccept() (restart func(), err error) {
	ln.once.Do(func() {
		ln.suspend = make(chan struct{})
		atomic.StoreUint32(&ln.stop, 1)
		restart = func() {
			close(ln.suspend)
			atomic.StoreUint32(&ln.stop, 0)
			ln.once = sync.Once{}
		}
	})
	if restart == nil {
		return nil, ErrAlreadyStopped
	}
	return restart, nil
}

type conn struct {
	net.Conn
	parent *gracefulListener
	once   sync.Once
}

func (c *conn) Close() error {
	err := c.Conn.Close()
	if err != syscall.EINTR {
		c.once.Do(c.parent.closeConn)
	}
	return err
}
