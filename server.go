package fastrpc

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
)

// HandlerCtx is an interface implementing context passed to Server.Handler
type HandlerCtx interface {
	// ConcurrencyLimitError must set the response
	// to 'concurrency limit exceeded' error.
	ConcurrencyLimitError(concurrency int)

	// Init must prepare ctx for reading the next request.
	Init(conn net.Conn, logger fasthttp.Logger)

	// ReadRequest must read request from br.
	ReadRequest(br *bufio.Reader) error

	// WriteResponse must write response to bw.
	WriteResponse(bw *bufio.Writer) error
}

// Server accepts rpc requests from Client.
type Server struct {
	// NewHandlerCtx must return new HandlerCtx
	NewHandlerCtx func() HandlerCtx

	// Handler must process incoming requests.
	//
	// The handler must return either ctx passed to the call
	// or new non-nil ctx.
	//
	// The handler may return ctx passed to the call only if the ctx
	// is no longer used after returning from the handler.
	// Otherwise new ctx must be returned.
	Handler func(ctx HandlerCtx) HandlerCtx

	Handshake        func(conn net.Conn) (net.Conn, error)
	HandshakeTimeout time.Duration

	// Concurrency is the maximum number of concurrent goroutines
	// with Server.Handler the server may run.
	//
	// DefaultConcurrency is used by default.
	Concurrency int

	// MaxBatchDelay is the maximum duration before ready responses
	// are sent to the client.
	//
	// Responses' batching may reduce network bandwidth usage and CPU usage.
	//
	// By default responses are sent immediately to the client.
	MaxBatchDelay time.Duration

	// Maximum duration for reading the full request (including body).
	//
	// This also limits the maximum lifetime for idle connections.
	//
	// By default request read timeout is unlimited.
	ReadTimeout time.Duration

	// Maximum duration for writing the full response (including body).
	//
	// By default response write timeout is unlimited.
	WriteTimeout time.Duration

	// ReadBufferSize is the size for read buffer.
	//
	// DefaultReadBufferSize is used by default.
	ReadBufferSize int

	// WriteBufferSize is the size for write buffer.
	//
	// DefaultWriteBufferSize is used by default.
	WriteBufferSize int

	// Logger, which is used by the Server.
	//
	// Standard logger from log package is used by default.
	Logger fasthttp.Logger

	// PipelineRequests enables requests' pipelining.
	//
	// Requests from a single client are processed serially
	// if is set to true.
	//
	// Enabling requests' pipelining may be useful in the following cases:
	//
	//   - if requests from a single client must be processed serially;
	//   - if the Server.Handler doesn't block and maximum throughput
	//     must be achieved for requests' processing.
	//
	// By default requests from a single client are processed concurrently.
	PipelineRequests bool

	workItemPool sync.Pool

	concurrencyCount uint32
}

func (s *Server) concurrency() int {
	concurrency := s.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}
	return concurrency
}

// Serve serves rpc requests accepted from the given listener.
func (s *Server) Serve(ln net.Listener) error {
	if s.Handler == nil {
		panic("BUG: Server.Handler must be set")
	}
	concurrency := s.concurrency()
	pipelineRequests := s.PipelineRequests
	for {
		conn, err := ln.Accept()
		if err != nil {
			if conn != nil {
				panic("BUG: net.Listener returned non-nil conn and non-nil error")
			}
			if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
				s.logger().Printf("fastrpc.Server: temporary error when accepting new connections: %s", netErr)
				time.Sleep(time.Second)
				continue
			}
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				s.logger().Printf("fastrpc.Server: permanent error when accepting new connections: %s", err)
				return err
			}
			return nil
		}
		if conn == nil {
			panic("BUG: net.Listener returned (nil, nil)")
		}

		if pipelineRequests {
			n := int(atomic.AddUint32(&s.concurrencyCount, 1))
			if n > concurrency {
				atomic.AddUint32(&s.concurrencyCount, ^uint32(0))
				s.logger().Printf("fastrpc.Server: concurrency limit exceeded: %d", concurrency)
				continue
			}
		}

		go func() {
			laddr := conn.LocalAddr().String()
			raddr := conn.RemoteAddr().String()
			if err := s.serveConn(conn); err != nil {
				s.logger().Printf("fastrpc.Server: error on connection %q<->%q: %s", laddr, raddr, err)
			}
			if pipelineRequests {
				atomic.AddUint32(&s.concurrencyCount, ^uint32(0))
			}
		}()
	}
}

func (s *Server) serveConn(conn net.Conn) error {
	realConn, br, bw, err := newBufioConn(conn, s.ReadBufferSize, s.WriteBufferSize, s.Handshake, s.HandshakeTimeout)
	if err != nil {
		conn.Close()
		return err
	}

	conn = realConn

	stopCh := make(chan struct{})

	pendingResponses := make(chan *serverWorkItem, s.concurrency())
	readerDone := make(chan error, 1)
	go func() {
		readerDone <- s.connReader(br, conn, pendingResponses, stopCh)
	}()

	writerDone := make(chan error, 1)
	go func() {
		writerDone <- s.connWriter(bw, conn, pendingResponses, stopCh)
	}()

	select {
	case err = <-readerDone:
		conn.Close()
		close(stopCh)
		<-writerDone
	case err = <-writerDone:
		conn.Close()
		close(stopCh)
		<-readerDone
	}
	return err
}

func (s *Server) connReader(br *bufio.Reader, conn net.Conn, pendingResponses chan<- *serverWorkItem, stopCh <-chan struct{}) error {
	logger := s.logger()
	concurrency := s.concurrency()
	pipelineRequests := s.PipelineRequests
	readTimeout := s.ReadTimeout

	var lastReadDeadline time.Time

	for {
		wi := s.acquireWorkItem()

		if readTimeout > 0 {
			// Optimization: update read deadline only if more than 25%
			// of the last read deadline exceeded.
			// See https://github.com/golang/go/issues/15133 for details.
			t := coarseTimeNow()
			if t.Sub(lastReadDeadline) > (readTimeout >> 2) {
				if err := conn.SetReadDeadline(t.Add(readTimeout)); err != nil {
					// do not panic here, since the error may
					// indicate that the connection is already closed
					return fmt.Errorf("cannot update read deadline: %s", err)
				}
				lastReadDeadline = t
			}
		}

		if n, err := io.ReadFull(br, wi.nonce[:]); err != nil {
			if n == 0 {
				// Ignore error if no bytes are read, since
				// the client may just close the connection.
				return nil
			}
			return fmt.Errorf("cannot read request ID: %s", err)
		}

		wi.ctx.Init(conn, logger)
		if err := wi.ctx.ReadRequest(br); err != nil {
			return fmt.Errorf("cannot read request: %s", err)
		}

		if pipelineRequests {
			s.handleRequest(wi, pendingResponses, stopCh)
		} else {
			n := int(atomic.AddUint32(&s.concurrencyCount, 1))
			if n > concurrency {
				atomic.AddUint32(&s.concurrencyCount, ^uint32(0))
				wi.ctx.ConcurrencyLimitError(concurrency)
				if !pushPendingResponse(pendingResponses, wi, stopCh) {
					return nil
				}
				continue
			}
			go func(wi *serverWorkItem) {
				s.handleRequest(wi, pendingResponses, stopCh)
				atomic.AddUint32(&s.concurrencyCount, ^uint32(0))
			}(wi)
		}
	}
}

func (s *Server) handleRequest(wi *serverWorkItem, pendingResponses chan<- *serverWorkItem, stopCh <-chan struct{}) {
	nonce, ctxNew := wi.nonce, s.Handler(wi.ctx)

	if isZeroNonce(nonce) {
		if ctxNew == wi.ctx {
			s.releaseWorkItem(wi)
		}
		return
	}

	if ctxNew != wi.ctx {
		if ctxNew == nil {
			panic("BUG: Server.Handler mustn't return nil")
		}

		wi = s.acquireWorkItem()
		wi.nonce = nonce
		wi.ctx = ctxNew
	}
	pushPendingResponse(pendingResponses, wi, stopCh)
}

func pushPendingResponse(pendingResponses chan<- *serverWorkItem, wi *serverWorkItem, stopCh <-chan struct{}) bool {
	select {
	case pendingResponses <- wi:
	default:
		select {
		case pendingResponses <- wi:
		case <-stopCh:
			return false
		}
	}
	return true
}

func (s *Server) connWriter(bw *bufio.Writer, conn net.Conn, pendingResponses <-chan *serverWorkItem, stopCh <-chan struct{}) error {
	var wi *serverWorkItem

	var (
		flushTimer    = getFlushTimer()
		flushCh       <-chan time.Time
		flushAlwaysCh = make(chan time.Time)
	)
	defer putFlushTimer(flushTimer)

	close(flushAlwaysCh)
	maxBatchDelay := s.MaxBatchDelay
	if maxBatchDelay < 0 {
		maxBatchDelay = 0
	}

	writeTimeout := s.WriteTimeout

	var lastWriteDeadline time.Time
	for {
		select {
		case wi = <-pendingResponses:
		default:
			select {
			case wi = <-pendingResponses:
			case <-stopCh:
				return nil
			case <-flushCh:
				if err := bw.Flush(); err != nil {
					return fmt.Errorf("cannot flush response data to client: %s", err)
				}
				flushCh = nil
				continue
			}
		}

		if writeTimeout > 0 {
			// Optimization: update write deadline only if more than 25%
			// of the last write deadline exceeded.
			// See https://github.com/golang/go/issues/15133 for details.
			t := coarseTimeNow()
			if t.Sub(lastWriteDeadline) > (writeTimeout >> 2) {
				if err := conn.SetWriteDeadline(t.Add(writeTimeout)); err != nil {
					// do not panic here, since the error may
					// indicate that the connection is already closed
					return fmt.Errorf("cannot update write deadline: %s", err)
				}
				lastWriteDeadline = t
			}
		}

		if _, err := bw.Write(wi.nonce[:]); err != nil {
			return fmt.Errorf("cannot write response ID: %s", err)
		}
		if err := wi.ctx.WriteResponse(bw); err != nil {
			return fmt.Errorf("cannot write response: %s", err)
		}

		s.releaseWorkItem(wi)

		// re-arm flush channel
		if flushCh == nil && len(pendingResponses) == 0 {
			if maxBatchDelay > 0 {
				resetFlushTimer(flushTimer, maxBatchDelay)
				flushCh = flushTimer.C
			} else {
				flushCh = flushAlwaysCh
			}
		}
	}
}

type serverWorkItem struct {
	ctx   HandlerCtx
	nonce [4]byte
}

func (s *Server) acquireWorkItem() *serverWorkItem {
	v := s.workItemPool.Get()
	if v == nil {
		return &serverWorkItem{
			ctx: s.NewHandlerCtx(),
		}
	}
	return v.(*serverWorkItem)
}

func (s *Server) releaseWorkItem(wi *serverWorkItem) {
	s.workItemPool.Put(wi)
}

var defaultLogger = log.New(os.Stderr, "", log.LstdFlags)

func (s *Server) logger() fasthttp.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return defaultLogger
}

func isZeroNonce(nonce [4]byte) bool {
	return nonce[0] == 0 && nonce[1] == 0 && nonce[2] == 0 && nonce[3] == 0
}
