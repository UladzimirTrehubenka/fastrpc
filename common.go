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

// CompressType is a compression type used for connections.
type CompressType byte

const (
	// CompressNone disables connection compression.
	//
	// CompressNone may be used in the following cases:
	//
	//   * If network bandwidth between client and server is unlimited.
	//   * If client and server are located on the same physical host.
	//   * If other CompressType values consume a lot of CPU resources.
	//
	CompressNone = CompressType(1)

	// CompressFlate uses compress/flate with default
	// compression level for connection compression.
	//
	// CompressFlate may be used in the following cases:
	//
	//     * If network bandwidth between client and server is limited.
	//     * If client and server are located on distinct physical hosts.
	//     * If both client and server have enough CPU resources
	//       for compression processing.
	//
	CompressFlate = CompressType(0)

	// CompressSnappy uses snappy compression.
	//
	// CompressSnappy vs CompressFlate comparison:
	//
	//     * CompressSnappy consumes less CPU resources.
	//     * CompressSnappy consumes more network bandwidth.
	//
	CompressSnappy = CompressType(2)
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
