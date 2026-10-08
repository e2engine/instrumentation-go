package http

import (
	"net/http"

	e2engine "github.com/e2engine/instrumentation-go"
)

const testExecutionIDHeader = "E2Engine-Test-Execution-ID"

// Inject returns a request containing the E2Engine test execution ID from its
// context as an HTTP header.
//
// The execution ID is read from the request context and, when present, is added
// to the outgoing HTTP request. This allows E2Engine to correlate the outgoing
// HTTP dependency call with the test execution that caused it.
//
// Inject clones the request before adding the header and does not modify the
// original request. If the context does not contain an E2Engine test execution
// ID, Inject returns the original request unchanged.
func Inject(r *http.Request) *http.Request {
	id := e2engine.TestExecutionID(r.Context())
	if id == "" {
		return r
	}

	r = r.Clone(r.Context())
	r.Header.Set(
		testExecutionIDHeader,
		id,
	)

	return r
}

type transport struct {
	next http.RoundTripper
}

func (t transport) RoundTrip(
	r *http.Request,
) (*http.Response, error) {
	return t.next.RoundTrip(Inject(r))
}

// Transport returns an HTTP transport that injects the E2Engine test execution
// ID into outgoing HTTP requests.
//
// Transport uses Inject for each request before passing it to the wrapped
// transport. The execution ID must therefore be present in the request context,
// typically because the context originated from an inbound request processed by
// Handler or Extract.
//
// Transport can wrap an existing HTTP transport:
//
//	client := &http.Client{Transport: Transport(customTransport)}
//
// When called without an argument, Transport uses http.DefaultTransport:
//
//	client := &http.Client{Transport: Transport()}
//
// If the request context does not contain an E2Engine test execution ID, the
// outgoing request is left unchanged. Transport accepts at most one transport
// and panics if more than one is supplied.
func Transport(next ...http.RoundTripper) http.RoundTripper {
	if len(next) > 1 {
		panic("e2engine http Transport accepts at most one transport")
	}

	var roundTripper http.RoundTripper
	if len(next) == 1 {
		roundTripper = next[0]
	}
	if roundTripper == nil {
		roundTripper = http.DefaultTransport
	}

	return transport{next: roundTripper}
}

// Extract returns a request whose context contains the E2Engine test execution
// ID from the incoming HTTP request.
//
// The execution ID is read from the incoming HTTP headers and, when present, is
// added to the request context using e2engine.WithTestExecutionID. This allows
// the execution ID to be carried through the application and propagated to
// outgoing dependency calls.
//
// If the request does not contain an E2Engine test execution ID, Extract
// returns the original request unchanged.
func Extract(r *http.Request) *http.Request {
	id := r.Header.Get(testExecutionIDHeader)
	if id == "" {
		return r
	}

	return r.WithContext(
		e2engine.WithTestExecutionID(
			r.Context(),
			id,
		),
	)
}

// Handler returns an HTTP handler that extracts the E2Engine test execution ID
// from incoming requests and adds it to their contexts.
//
// Handler uses Extract before passing each request to the wrapped handler. This
// allows the execution ID to be carried through the application using
// context.Context and later propagated to outgoing dependency calls using
// Inject, Transport, or another E2Engine integration.
//
// Handler can wrap an existing HTTP handler:
//
//	http.ListenAndServe(":8080", Handler(mux))
//
// When called without an argument, Handler uses http.DefaultServeMux:
//
//	http.ListenAndServe(":8080", Handler())
//
// If the incoming request does not contain an E2Engine test execution ID, its
// context is left unchanged. Handler accepts at most one handler and panics if
// more than one is supplied.
func Handler(next ...http.Handler) http.Handler {
	if len(next) > 1 {
		panic("e2engine http Handler accepts at most one handler")
	}

	var handler http.Handler
	if len(next) == 1 {
		handler = next[0]
	}
	if handler == nil {
		handler = http.DefaultServeMux
	}

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, Extract(r))
		},
	)
}

// TestExecutionID returns the E2Engine test execution ID from the HTTP headers.
//
// If the headers do not contain an E2Engine test execution ID, TestExecutionID
// returns an empty string.
func TestExecutionID(
	header http.Header,
) string {
	return header.Get(testExecutionIDHeader)
}

// WithTestExecutionID returns a copy of the HTTP headers containing the
// provided E2Engine test execution ID.
//
// If the headers already contain an E2Engine test execution ID, it is replaced.
// If id is empty, any existing execution ID is removed. The supplied headers
// are not modified.
func WithTestExecutionID(
	header http.Header,
	id string,
) http.Header {
	result := header.Clone()

	if id == "" {
		result.Del(testExecutionIDHeader)
		return result
	}

	result.Set(
		testExecutionIDHeader,
		id,
	)

	return result
}

// WithoutTestExecutionID returns a copy of the HTTP headers without the
// E2Engine test execution ID.
//
// The supplied headers are not modified.
func WithoutTestExecutionID(
	header http.Header,
) http.Header {
	result := header.Clone()
	result.Del(testExecutionIDHeader)

	return result
}
