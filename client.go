package fastrpc

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/valyala/fasthttp"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// RequestWriter is an interface for writing rpc request to buffered writer.
type RequestWriter interface {
	// WriteRequest must write request to bw.
	WriteRequest(bw *bufio.Writer) error
}

// ResponseReader is an interface for reading rpc response from buffered reader.
type ResponseReader interface {
	// ReadResponse must read response from br.
	ReadResponse(br *bufio.Reader) error
}

// Client sends rpc requests to the Server over a single connection.
//
// Use multiple clients for establishing multiple connections to the server
// if a single connection processing consumes 100% of a single CPU core
// on either multi-core client or server.
type Client struct {
	// NewResponse must return new response object.
	NewResponse func() ResponseReader

	// Addr is the Server address to connect to.
	Addr string

	// Dial is a custom function used for connecting to the Server.
	//
	// fasthttp.Dial is used by default.
	Dial func(addr string) (net.Conn, error)

	Handshake        func(conn net.Conn) (net.Conn, error)
	HandshakeTimeout time.Duration

	// MaxPendingRequests is the maximum number of pending requests
	// the client may issue until the server responds to them.
	//
	// DefaultMaxPendingRequests is used by default.
	MaxPendingRequests int

	// MaxBatchDelay is the maximum duration before pending requests
	// are sent to the server.
	//
	// Requests' batching may reduce network bandwidth usage and CPU usage.
	//
	// By default requests are sent immediately to the server.
	MaxBatchDelay time.Duration

	// Maximum duration for full response reading (including body).
	//
	// This also limits idle connection lifetime duration.
	//
	// By default response read timeout is unlimited.
	ReadTimeout time.Duration

	// Maximum duration for full request writing (including body).
	//
	// By default request write timeout is unlimited.
	WriteTimeout time.Duration

	// ReadBufferSize is the size for read buffer.
	//
	// DefaultReadBufferSize is used by default.
	ReadBufferSize int

	// WriteBufferSize is the size for write buffer.
	//
	// DefaultWriteBufferSize is used by default.
	WriteBufferSize int

	// Prioritizes new requests over old requests if MaxPendingRequests pending
	// requests is reached.
	PrioritizeNewRequests bool

	once sync.Once

	lastErrMu sync.Mutex
	lastErr   error

	pendingRequests chan *clientWorkItem

	pendingResponses   map[uint32]*clientWorkItem
	pendingResponsesMu sync.Mutex

	pendingRequestsCount uint32

	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	conn   net.Conn
	connMu sync.Mutex
}

var (
	// ErrTimeout is returned from timed out calls.
	ErrTimeout = fasthttp.ErrTimeout

	// ErrPendingRequestsOverflow is returned when Client cannot send
	// more requests to the server due to Client.MaxPendingRequests limit.
	ErrPendingRequestsOverflow = errors.New("pending requests overflowed")
)

// SendNowait schedules the given request for sending to the server
// set in Client.Addr.
//
// req cannot be used after SendNowait returns and until releaseReq is called.
// releaseReq is called when the req is no longer needed and may be re-used.
//
// req cannot be re-used if releaseReq is nil.
//
// Returns true if the request is successfully scheduled for sending,
// otherwise returns false.
//
// Response for the given request is ignored.
func (c *Client) SendNowait(req RequestWriter, releaseReq func(req RequestWriter)) bool {
	c.once.Do(c.init)

	// Do not track 'nowait' request as a pending request, since it
	// has no response.

	wi := acquireClientWorkItem()
	wi.req = req
	wi.releaseReq = releaseReq
	wi.deadline = coarseTimeNow().Add(10 * time.Second)
	if err := c.enqueueWorkItem(wi); err != nil {
		releaseClientWorkItem(wi)
		return false
	}
	return true
}

// DoDeadline sends the given request to the server set in Client.Addr.
//
// ErrTimeout is returned if the server didn't return response until
// the given deadline.
func (c *Client) DoDeadline(req RequestWriter, resp ResponseReader, deadline time.Time) error {
	c.once.Do(c.init)

	n := c.incPendingRequests()
	defer c.decPendingRequests()

	if n >= c.maxPendingRequests() {
		c.decPendingRequests()
		return c.getError(ErrPendingRequestsOverflow)
	}

	wi := acquireClientWorkItem()
	defer releaseClientWorkItem(wi)

	wi.req = req
	wi.resp = resp
	wi.deadline = deadline

	if err := c.enqueueWorkItem(wi); err != nil {
		return c.getError(err)
	}

	return <-wi.done
}

func (c *Client) enqueueWorkItem(wi *clientWorkItem) error {
	select {
	case c.pendingRequests <- wi:
		return nil
	default:
		if !c.PrioritizeNewRequests {
			return ErrPendingRequestsOverflow
		}

		// slow path
		select {
		case old := <-c.pendingRequests:
			c.doneError(old, ErrPendingRequestsOverflow)
			select {
			case c.pendingRequests <- wi:
				return nil
			default:
				return ErrPendingRequestsOverflow
			}
		default:
			return ErrPendingRequestsOverflow
		}
	}
}

func (c *Client) maxPendingRequests() int {
	maxPendingRequests := c.MaxPendingRequests
	if maxPendingRequests <= 0 {
		maxPendingRequests = DefaultMaxPendingRequests
	}
	return maxPendingRequests
}

func (c *Client) Close() {
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()

	c.stopOnce.Do(func() {
		if conn != nil {
			conn.Close()
		}

		close(c.stop)
	})

	c.wg.Wait()
}

func (c *Client) init() {
	if c.NewResponse == nil {
		panic("BUG: Client.NewResponse cannot be nil")
	}

	n := c.maxPendingRequests()
	c.pendingRequests = make(chan *clientWorkItem, n)
	c.pendingResponses = make(map[uint32]*clientWorkItem, n)

	c.stop = make(chan struct{})
	c.wg.Add(2)

	go c.unblockStaleItems()
	go c.worker()
}

func (c *Client) unblockStaleItems() {
	defer c.wg.Done()

	sleepDuration := 10 * time.Millisecond

	for {
		select {
		case <-c.stop:
			return
		case <-time.After(sleepDuration):
		}

		if c.unblockStaleRequests() || c.unblockStaleResponses() {
			sleepDuration = time.Duration(0.7 * float64(sleepDuration))
			if sleepDuration < 10*time.Millisecond {
				sleepDuration = 10 * time.Millisecond
			}
		} else {
			sleepDuration = time.Duration(1.5 * float64(sleepDuration))
			if sleepDuration > time.Second {
				sleepDuration = time.Second
			}
		}
	}
}

func (c *Client) unblockStaleRequests() bool {
	found := false
	n := len(c.pendingRequests)
	t := time.Now()
	for i := 0; i < n; i++ {
		select {
		case wi := <-c.pendingRequests:
			if t.After(wi.deadline) {
				c.doneError(wi, ErrTimeout)
				found = true
			} else {
				if err := c.enqueueWorkItem(wi); err != nil {
					c.doneError(wi, err)
				}
			}
		default:
			return found
		}
	}
	return found
}

func (c *Client) unblockStaleResponses() bool {
	now, unblocked := time.Now(), false

	c.pendingResponsesMu.Lock()
	defer c.pendingResponsesMu.Unlock()

	for nonce, wi := range c.pendingResponses {
		if now.After(wi.deadline) {
			delete(c.pendingResponses, nonce)
			c.doneError(wi, ErrTimeout)
			unblocked = true
		}
	}

	return unblocked
}

// PendingRequests returns the number of pending requests at the moment.
//
// This function may be used either for informational purposes
// or for load balancing purposes.
func (c *Client) PendingRequests() int {
	return int(atomic.LoadUint32(&c.pendingRequestsCount))
}

func (c *Client) incPendingRequests() int {
	return int(atomic.AddUint32(&c.pendingRequestsCount, 1))
}

func (c *Client) decPendingRequests() {
	atomic.AddUint32(&c.pendingRequestsCount, ^uint32(0))
}

func (c *Client) worker() {
	defer c.wg.Done()

	dial := c.Dial
	if dial == nil {
		dial = fasthttp.Dial
	}

	for {
		var wi *clientWorkItem

		select {
		case <-c.stop:
			return
		case wi = <-c.pendingRequests:
		}

		if err := c.enqueueWorkItem(wi); err != nil {
			c.doneError(wi, err)
		}

		conn, err := dial(c.Addr)
		if err != nil {
			c.setLastError(fmt.Errorf("cannot connect to %q: %w", c.Addr, err))

			select {
			case <-c.stop:
				return
			case <-time.After(1 * time.Second):
			}

			continue
		}

		c.connMu.Lock()
		c.conn = realConn
		c.connMu.Unlock()

		laddr := conn.LocalAddr().String()
		raddr := conn.RemoteAddr().String()

		err = c.serveConn(conn)

		if err == nil {
			c.setLastError(fmt.Errorf("%s<->%s: connection closed by server", laddr, raddr))
		} else {
			c.setLastError(fmt.Errorf("%s<->%s: %w", laddr, raddr, err))
		}

		c.pendingResponsesMu.Lock()
		for nonce, wi := range c.pendingResponses {
			c.doneError(wi, nil)
			delete(c.pendingResponses, nonce)
		}
		c.pendingResponsesMu.Unlock()
	}
}

func (c *Client) serveConn(conn net.Conn) error {
	realConn, br, bw, err := newBufioConn(conn, c.ReadBufferSize, c.WriteBufferSize, c.Handshake, c.HandshakeTimeout)
	if err != nil {
		conn.Close()

		select {
		case <-c.stop:
		case <-time.After(1 * time.Second):
		}

		return err
	}

	c.connMu.Lock()
	c.conn = realConn
	c.connMu.Unlock()

	readerDone := make(chan error, 1)
	go func() {
		readerDone <- c.connReader(br, realConn)
	}()

	writerDone := make(chan error, 1)
	stopWriterCh := make(chan struct{})
	go func() {
		writerDone <- c.connWriter(bw, realConn, stopWriterCh)
	}()

	select {
	case err = <-readerDone:
		close(stopWriterCh)
		realConn.Close()
		<-writerDone
	case err = <-writerDone:
		realConn.Close()
		<-readerDone
	}

	return err
}

func (c *Client) connWriter(bw *bufio.Writer, conn net.Conn, stopCh <-chan struct{}) error {
	var (
		wi  *clientWorkItem
		buf [4]byte
	)

	var (
		flushTimer    = getFlushTimer()
		flushCh       <-chan time.Time
		flushAlwaysCh = make(chan time.Time)
	)
	defer putFlushTimer(flushTimer)

	close(flushAlwaysCh)
	maxBatchDelay := c.MaxBatchDelay
	if maxBatchDelay < 0 {
		maxBatchDelay = 0
	}

	writeTimeout := c.WriteTimeout
	var lastWriteDeadline time.Time
	var nextNonce uint32
	for {
		select {
		case wi = <-c.pendingRequests:
		default:
			// slow path
			select {
			case wi = <-c.pendingRequests:
			case <-stopCh:
				return nil
			case <-flushCh:
				if err := bw.Flush(); err != nil {
					return fmt.Errorf("cannot flush requests data to the server: %w", err)
				}
				flushCh = nil
				continue
			}
		}

		t := coarseTimeNow()
		if t.After(wi.deadline) {
			c.doneError(wi, ErrTimeout)
			continue
		}

		nonce := uint32(0)
		if wi.resp != nil {
			nextNonce++
			if nextNonce == 0 {
				nextNonce = 1
			}
			nonce = nextNonce
		}

		if writeTimeout > 0 {
			if t.Sub(lastWriteDeadline) > (writeTimeout >> 2) {
				if err := conn.SetWriteDeadline(t.Add(writeTimeout)); err != nil {
					err = fmt.Errorf("cannot update write deadline: %w", err)
					c.doneError(wi, err)
					return err
				}
				lastWriteDeadline = t
			}
		}

		b := appendUint32(buf[:0], nonce)
		if _, err := bw.Write(b); err != nil {
			err = fmt.Errorf("cannot send request ID to the server: %w", err)
			c.doneError(wi, err)
			return err
		}

		if err := wi.req.WriteRequest(bw); err != nil {
			err = fmt.Errorf("cannot send request to the server: %w", err)
			c.doneError(wi, err)
			return err
		}

		if wi.resp == nil {
			releaseClientWorkItem(wi)
		} else {
			c.pendingResponsesMu.Lock()
			if _, ok := c.pendingResponses[nonce]; ok {
				c.pendingResponsesMu.Unlock()
				err := fmt.Errorf("request ID overflow. id=%d", nonce)
				c.doneError(wi, err)
				return err
			}
			c.pendingResponses[nonce] = wi
			c.pendingResponsesMu.Unlock()
		}

		// re-arm flush channel
		if flushCh == nil && len(c.pendingRequests) == 0 {
			if maxBatchDelay > 0 {
				resetFlushTimer(flushTimer, maxBatchDelay)
				flushCh = flushTimer.C
			} else {
				flushCh = flushAlwaysCh
			}
		}
	}
}

func (c *Client) connReader(br *bufio.Reader, conn net.Conn) error {
	var (
		buf  [4]byte
		resp ResponseReader
	)

	zeroResp := c.NewResponse()

	readTimeout := c.ReadTimeout
	var lastReadDeadline time.Time
	for {
		if readTimeout > 0 {
			t := coarseTimeNow()

			if t.Sub(lastReadDeadline) > (readTimeout >> 2) {
				if err := conn.SetReadDeadline(t.Add(readTimeout)); err != nil {
					return fmt.Errorf("cannot update read deadline: %w", err)
				}
				lastReadDeadline = t
			}
		}

		if _, err := io.ReadFull(br, buf[:]); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("cannot read response ID: %w", err)
		}

		nonce := bytes2Uint32(buf)

		c.pendingResponsesMu.Lock()
		wi := c.pendingResponses[nonce]
		delete(c.pendingResponses, nonce)
		c.pendingResponsesMu.Unlock()

		resp = nil
		if wi != nil {
			resp = wi.resp
		}
		if resp == nil {
			resp = zeroResp
		}

		if err := resp.ReadResponse(br); err != nil {
			err = fmt.Errorf("cannot read response with ID %d: %w", nonce, err)
			if wi != nil {
				c.doneError(wi, err)
			}
			return err
		}

		if wi != nil {
			if wi.resp == nil {
				panic("BUG: clientWorkItem.resp must be non-nil")
			}
			wi.done <- nil
		}
	}
}

func (c *Client) doneError(wi *clientWorkItem, err error) {
	if wi.resp != nil {
		wi.done <- c.getError(err)
	} else {
		releaseClientWorkItem(wi)
	}
}

func (c *Client) getError(err error) error {
	c.lastErrMu.Lock()
	lastErr := c.lastErr
	c.lastErrMu.Unlock()
	if lastErr != nil {
		return lastErr
	}
	return err
}

func (c *Client) setLastError(err error) {
	c.lastErrMu.Lock()
	c.lastErr = err
	c.lastErrMu.Unlock()
}

type clientWorkItem struct {
	req        RequestWriter
	resp       ResponseReader
	releaseReq func(req RequestWriter)
	deadline   time.Time
	done       chan error
}

func acquireClientWorkItem() *clientWorkItem {
	v := clientWorkItemPool.Get()
	if v == nil {
		v = &clientWorkItem{
			done: make(chan error, 1),
		}
	}
	wi := v.(*clientWorkItem)
	if len(wi.done) != 0 {
		panic("BUG: clientWorkItem.done must be empty")
	}
	return wi
}

func releaseClientWorkItem(wi *clientWorkItem) {
	if len(wi.done) != 0 {
		panic("BUG: clientWorkItem.done must be empty")
	}
	if wi.releaseReq != nil {
		if wi.resp != nil {
			panic("BUG: clientWorkItem.resp must be nil")
		}
		wi.releaseReq(wi.req)
	}
	wi.req = nil
	wi.resp = nil
	wi.releaseReq = nil
	clientWorkItemPool.Put(wi)
}

var clientWorkItemPool sync.Pool

func appendUint32(b []byte, n uint32) []byte {
	return append(b, byte(n), byte(n>>8), byte(n>>16), byte(n>>24))
}

func bytes2Uint32(b [4]byte) uint32 {
	return (uint32(b[3]) << 24) | (uint32(b[2]) << 16) | (uint32(b[1]) << 8) | uint32(b[0])
}
