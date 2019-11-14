package graceful

import (
	"sync/atomic"
)

// cancellable waiter

type waiter struct {
	c       uint64
	waiting uint32
	done    chan struct{}
	cancel  chan struct{}
}

func (w *waiter) Add() {
	atomic.AddUint64(&w.c, 1)
}

func (w *waiter) Done() {
	newCount := atomic.AddUint64(&w.c, ^uint64(0)) // decrement
	if newCount == 0 {
		waiting := atomic.LoadUint32(&w.waiting)
		if waiting != 0 { // Loadして、0じゃなかったらクローズ
			close(w.done)
		}
	}
}

// restartができるような仕様にしたい
func (w *waiter) Wait() {
	waiting := atomic.AddUint32(&w.waiting, 1)
	if atomic.LoadUint64(&w.c) == 0 && waiting == 1 {
		close(w.done)
	}

	select {
	case <-w.done:
	case <-w.cancel:
	}
}

func (w *waiter) CancelWait() {
	close(w.cancel)
}

func (w *waiter) Reset() {
	w.done = make(chan struct{})
	w.cancel = make(chan struct{})
	// w.c = 0
	atomic.StoreUint32(&w.waiting, 0)
}
