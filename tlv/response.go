package tlv

import (
	"bufio"
	"fmt"
	"sync"
)

// Response is a TLV response.
type Response struct {
	value  []byte
	header [4]byte
}

// Reset resets the given response.
func (r *Response) Reset() {
	r.value = r.value[:0]
}

func (r *Response) Value() []byte {
	return r.value
}

// Write appends p to the response value.
//
// It implements io.Writer.
func (r *Response) Write(p []byte) (int, error) {
	r.Append(p)
	return len(p), nil
}

// Append appends p to the response value.
func (r *Response) Append(p []byte) {
	r.value = append(r.value, p...)
}

// Swap swaps the given value with the response's value.
//
// It is forbidden accessing the swapped value after the call.
func (r *Response) Swap(value []byte) []byte {
	v := r.value
	r.value = value
	return v
}

// WriteResponse writes the response to bw.
func (r *Response) WriteResponse(bw *bufio.Writer) error {
	if err := writeBytes(bw, r.value, r.header[:]); err != nil {
		return fmt.Errorf("cannot write response value: %s", err)
	}
	return nil
}

// ReadResponse reads the response from br.
//
// It implements fastrpc.ReadResponse.
func (r *Response) ReadResponse(br *bufio.Reader) error {
	var err error
	r.value, err = readBytes(br, r.value[:0], r.header[:])
	if err != nil {
		return fmt.Errorf("cannot read request value: %s", err)
	}
	return nil
}

// AcquireResponse acquires new response.
func AcquireResponse() *Response {
	v := responsePool.Get()
	if v == nil {
		v = &Response{}
	}
	return v.(*Response)
}

// ReleaseResponse releases the given response.
func ReleaseResponse(r *Response) {
	r.Reset()
	responsePool.Put(r)
}

var responsePool sync.Pool
