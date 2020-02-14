package fastrpc

import (
	"bufio"
	"net"
	"sync"
	"time"
)

const (
	// DefaultMaxPendingRequests is the default number of pending requests
	// a single Client may queue before sending them to the server.
	//
	// This parameter may be overridden by Client.MaxPendingRequests.
	DefaultMaxPendingRequests = 1000

	// DefaultConcurrency is the default maximum number of concurrent
	// Server.Handler goroutines the server may run.
	DefaultConcurrency = 10000
)

const (
	// DefaultReadBufferSize is the default size for read buffers.
	DefaultReadBufferSize = 64 * 1024

	// DefaultWriteBufferSize is the default size for write buffers.
	DefaultWriteBufferSize = 64 * 1024
)

func newBufioConn(conn net.Conn, readBufferSize, writeBufferSize int) (*bufio.Reader, *bufio.Writer, error) {
	if readBufferSize <= 0 {
		readBufferSize = DefaultReadBufferSize
	}

	br := bufio.NewReaderSize(conn, readBufferSize)

	if writeBufferSize <= 0 {
		writeBufferSize = DefaultWriteBufferSize
	}

	bw := bufio.NewWriterSize(conn, writeBufferSize)

	return br, bw, nil
}

func getFlushTimer() *time.Timer {
	v := flushTimerPool.Get()
	if v == nil {
		return time.NewTimer(time.Hour * 24)
	}
	t := v.(*time.Timer)
	resetFlushTimer(t, time.Hour*24)
	return t
}

func putFlushTimer(t *time.Timer) {
	stopFlushTimer(t)
	flushTimerPool.Put(t)
}

func resetFlushTimer(t *time.Timer, d time.Duration) {
	stopFlushTimer(t)
	t.Reset(d)
}

func stopFlushTimer(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}

var flushTimerPool sync.Pool
