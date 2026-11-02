package http

import (
	"net/http"

	e2engine "github.com/e2engine/instrumentation-go"
)

const testExecutionIDHeader = "E2Engine-Test-Execution-ID"

// Inject returns a request containing a valid E2Engine test execution ID
// as an HTTP header.
//
// Exactly one valid execution ID header value takes precedence over the
// execution ID stored in the request context.
//
// If the existing header is absent, invalid, or contains multiple values, a
// valid execution ID from the context is used instead. If neither source
// provides a valid ID, any invalid or multiple execution ID values are removed.
//
// Inject never modifies the original request. If no changes are necessary,
// the original request is returned unchanged.
func Inject(r *http.Request) *http.Request {
	values, exists := r.Header[http.CanonicalHeaderKey(testExecutionIDHeader)]

	if exists && len(values) == 1 &&
		e2engine.ValidTestExecutionID(values[0]) {
		return r
	}

	contextID := e2engine.TestExecutionID(r.Context())
	validContextID := e2engine.ValidTestExecutionID(contextID)

	if !exists && !validContextID {
		return r
	}

	result := r.Clone(r.Context())

	if validContextID {
		if result.Header == nil {
			result.Header = make(http.Header)
		}
		result.Header.Set(testExecutionIDHeader, contextID)
	} else {
		result.Header.Del(testExecutionIDHeader)
	}

	return result
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
// transport. Exactly one valid execution ID header value takes precedence over
// the request context. If the header is absent, invalid, or contains multiple
// values, Transport uses a valid execution ID from the request context,
// typically originating from an inbound request processed by Handler or Extract.
//
// Transport can wrap an existing HTTP transport:
//
//	client := &http.Client{Transport: Transport(customTransport)}
//
// When called without an argument, Transport uses http.DefaultTransport:
//
//	client := &http.Client{Transport: Transport()}
//
// If neither the request headers nor its context contain a valid execution
// ID, any invalid or multiple execution ID values are removed. If no changes
// are necessary, the original request is passed to the wrapped transport.
// Transport accepts at most one transport and panics if more than one is supplied.
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
// The execution ID is read from the incoming HTTP headers and, when valid,
// is added to the request context using e2engine.WithTestExecutionID.
//
// If the header is missing, invalid, or contains multiple values, Extract
// returns the original request unchanged.
func Extract(r *http.Request) *http.Request {
	values := r.Header.Values(testExecutionIDHeader)

	if len(values) != 1 ||
		!e2engine.ValidTestExecutionID(values[0]) {
		return r
	}

	return r.WithContext(
		e2engine.WithTestExecutionID(
			r.Context(),
			values[0],
		),
	)
}

// Handler returns an HTTP handler that extracts the E2Engine test execution ID
// from incoming requests and adds it to their contexts.
//
// Handler uses Extract before passing each request to the wrapped handler.
// This allows the execution ID to be carried through the application using
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
// If the incoming request does not contain a valid E2Engine test execution ID,
// its context is left unchanged. Handler accepts at most one handler and panics
// if more than one is supplied.
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
// returns an empty string. The returned value is not validated.
func TestExecutionID(
	header http.Header,
) string {
	return header.Get(testExecutionIDHeader)
}

// WithTestExecutionID returns a copy of the HTTP headers containing the
// provided E2Engine test execution ID.
//
// If id is valid, it replaces any existing execution ID. If id is empty or
// invalid, any existing execution ID is removed. The supplied headers are
// not modified.
func WithTestExecutionID(
	header http.Header,
	id string,
) http.Header {
	result := header.Clone()

	if !e2engine.ValidTestExecutionID(id) {
		result.Del(testExecutionIDHeader)
		return result
	}

	if result == nil {
		result = make(http.Header)
	}
	result.Set(testExecutionIDHeader, id)

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
