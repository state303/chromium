package testserver

import (
	"fmt"
	"github.com/state303/chromium/internal/test/testfile"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// HttpRequest is a wrapper for *http.Request that provides time recorded when the request has arrived.
type HttpRequest struct {
	*http.Request
	time time.Time
}

// GetTime returns time.Time that are recorded when received request.
func (r *HttpRequest) GetTime() time.Time {
	return r.time
}

// HandleFunc is a request handler function that has access to accumulated HttpRequest.
// Whether the initial requests being nil or empty depends on the test implementation.
type HandleFunc func(requests []*HttpRequest, w http.ResponseWriter, r *http.Request)

// NewServer returns a server with given HandleFunc.
func NewServer(h HandleFunc) *TestServer {
	serverRequests := make([]*HttpRequest, 0)
	handler := &HttpHandler{handleFunc: h}
	server := &TestServer{httptest.NewServer(handler), serverRequests}
	handler.server = server
	return server
}

func rotater[T any](items ...T) func() T {
	idx, l := 0, &sync.Mutex{}
	max := len(items) - 1
	return func() T {
		l.Lock()
		defer l.Unlock()
		item := items[idx]
		if max > 0 {
			if idx < max {
				idx++
			} else {
				idx = 0
			}
		}
		return item
	}
}

// WithRotatingResponses creates a server with rotating response.
// If no payload is given, the payload will be
func WithRotatingResponses(t *testing.T, payload ...[]byte) *TestServer {
	var getPayload func() []byte
	if payload == nil || len(payload) == 0 {
		getPayload = rotater(testfile.BlankHTML)
	} else {
		getPayload = rotater(payload...)
	}
	return NewServer(func(requests []*HttpRequest, w http.ResponseWriter, r *http.Request) {
		if err := writeResponse(w, getPayload()); err != nil {
			t.Fatal(err.Error())
		}
	})
}

// WithResponseAfter returns a test server that responds differently after n times of requests.
// Do note that if n is set to 5, 5th request will receive initial.
func WithResponseAfter(t *testing.T, initial, after []byte, n int) *TestServer {
	i, a := emptyHtmlIfEmpty(initial), emptyHtmlIfEmpty(after)

	handleFunc := func(requests []*HttpRequest, w http.ResponseWriter, r *http.Request) {
		var payload []byte
		if len(requests) <= n {
			payload = i
		} else {
			payload = a
		}
		if err := writeResponse(w, payload); err != nil {
			t.Fatal(err.Error())
		}
	}

	return NewServer(handleFunc)
}

func writeResponse(w http.ResponseWriter, payload []byte) error {
	if wrote, err := w.Write(payload); err != nil {
		return err
	} else if expected := len(payload); wrote != expected {
		return fmt.Errorf("server wrote unexpected length of request. got: %+v, want: %+v", wrote, expected)
	}

	return nil
}

// emptyHtmlIfEmpty returns either blank html if empty or nil, depending on given parameter.
func emptyHtmlIfEmpty(in []byte) []byte {
	if in == nil || len(in) == 0 {
		return testfile.BlankHTML
	}
	return in
}

// TestServer is a wrapper for httptest.Server such that it could support accumulation of incoming requests.
type TestServer struct {
	*httptest.Server
	requests []*HttpRequest
}

// Requests returns accumulated requests that this server instance have received.
func (f *TestServer) Requests() []*HttpRequest {
	return f.requests
}

// HttpHandler is an implementation of http.Handler to be used for testing.
type HttpHandler struct {
	server     *TestServer
	handleFunc HandleFunc
}

// ServeHTTP accumulates incoming request into server.requests, then pass it down to its HandleFunc to handle at once.
func (h *HttpHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	h.server.requests = append(h.server.requests, &HttpRequest{request, time.Now()})
	h.handleFunc(h.server.requests, writer, request)
}
