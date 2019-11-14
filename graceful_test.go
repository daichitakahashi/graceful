package graceful

import (
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/nettest"
)

func newListener() Listener {
	rawLn, err := nettest.NewLocalListener("unix")
	if err != nil {
		panic("error on establishing local listener")
	}
	return Upgrade(rawLn)
}

func executeClients(addr string, num int32) {
	for i := int32(0); i < num; i++ {
		go func() {
			err := clientDo(addr)
			if err != nil {
				return
			}
		}()
	}
}

func executeAccepts(ln Listener, sleep time.Duration, num int32, count *int32) {
	for i := int32(0); i < num; i++ {
		conn, err := ln.Accept()
		if err != nil {
			break
		}
		go sleepHandler(conn, sleep, count) // time
	}
}

func TestGracefulShutdown(t *testing.T) {
	ln := newListener()
	defer ln.Close()

	finishedN := int32(0)
	connections := int32(5)

	// client
	go executeClients(ln.Addr().String(), connections)
	// server
	executeAccepts(ln, time.Millisecond*500, connections, &finishedN)

	err := ln.GracefulShutdown(-1) // no timeout
	if err != nil {
		panic(err)
	}
	if finishedN != connections {
		panic("not graceful")
	}
}

func sleepHandler(conn net.Conn, d time.Duration, count *int32) {
	defer conn.Close()
	time.Sleep(d)
	atomic.AddInt32(count, 1)
}

func clientDo(addr string) error {
	conn, err := net.Dial("unix", addr)
	if err != nil {
		return errors.New("dial failed")
	}
	defer conn.Close()
	bs := make([]byte, 10)
	conn.Read(bs)

	return nil
}

func TestGracefulTimeout(t *testing.T) {
	ln := newListener()
	defer ln.Close()

	finishedN := int32(0)
	connections := int32(5)

	// client
	go executeClients(ln.Addr().String(), connections)
	// server
	executeAccepts(ln, time.Millisecond*500, connections, &finishedN)

	err := ln.GracefulShutdown(time.Millisecond * 200)
	if err != ErrTimeout {
		panic("not timeout")
	}
	if finishedN == 5 {
		panic("not cancelled")
	}
}

func TestShutdown(t *testing.T) {
	ln := newListener()
	defer ln.Close()

	finishedN := int32(0)
	connections := int32(5)

	// client
	go executeClients(ln.Addr().String(), connections)
	// server
	executeAccepts(ln, time.Millisecond*500, connections, &finishedN)

	time.Sleep(time.Millisecond * 200)
	err := ln.Shutdown()
	if err != nil {
		panic(err)
	}
	if finishedN == 5 {
		panic("not shutdowned")
	}
}
