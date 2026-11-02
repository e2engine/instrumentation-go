package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	e2engine "github.com/e2engine/instrumentation-go"
)

var (
	validID   = strings.Repeat("a1b2c3d4", 8)
	anotherID = strings.Repeat("b2c3d4e5", 8)
)

func TestExtract(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		existingID  string
		wantID      string
		wantSameReq bool
	}{
		{
			name:   "valid execution ID",
			header: validID,
			wantID: validID,
		},
		{
			name:        "no execution ID",
			wantSameReq: true,
		},
		{
			name:        "invalid execution ID",
			header:      "execution-123",
			wantSameReq: true,
		},
		{
			name:        "uppercase execution ID",
			header:      strings.Repeat("A", 64),
			wantSameReq: true,
		},
		{
			name:        "execution ID too long",
			header:      strings.Repeat("a", 65),
			wantSameReq: true,
		},
		{
			name:        "existing ID with no header",
			existingID:  anotherID,
			wantID:      anotherID,
			wantSameReq: true,
		},
		{
			name:        "existing ID with invalid header",
			header:      "invalid",
			existingID:  anotherID,
			wantID:      anotherID,
			wantSameReq: true,
		},
		{
			name:       "valid header overrides existing ID",
			header:     validID,
			existingID: anotherID,
			wantID:     validID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)

			if tt.existingID != "" {
				request = request.WithContext(
					e2engine.WithTestExecutionID(
						request.Context(),
						tt.existingID,
					),
				)
			}

			if tt.header != "" {
				request.Header.Set(testExecutionIDHeader, tt.header)
			}

			got := Extract(request)

			if id := e2engine.TestExecutionID(got.Context()); id != tt.wantID {
				t.Fatalf(
					"TestExecutionID() = %q, want %q",
					id,
					tt.wantID,
				)
			}

			if tt.wantSameReq && got != request {
				t.Fatal("Extract() returned a new request without a valid execution ID")
			}

			if !tt.wantSameReq && got == request {
				t.Fatal("Extract() did not copy the request for a valid execution ID")
			}
		})
	}
}

func TestHandler(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		wantID string
	}{
		{
			name:   "valid execution ID",
			id:     validID,
			wantID: validID,
		},
		{
			name: "no execution ID",
		},
		{
			name: "invalid execution ID",
			id:   "execution-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotID string

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotID = e2engine.TestExecutionID(r.Context())
				w.WriteHeader(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.id != "" {
				request.Header.Set(testExecutionIDHeader, tt.id)
			}

			response := httptest.NewRecorder()

			Handler(next).ServeHTTP(response, request)

			if gotID != tt.wantID {
				t.Fatalf(
					"TestExecutionID() = %q, want %q",
					gotID,
					tt.wantID,
				)
			}

			if response.Code != http.StatusNoContent {
				t.Fatalf(
					"status = %d, want %d",
					response.Code,
					http.StatusNoContent,
				)
			}
		})
	}
}

func TestHandler_DefaultServeMux(t *testing.T) {
	previous := http.DefaultServeMux
	t.Cleanup(func() {
		http.DefaultServeMux = previous
	})

	http.DefaultServeMux = http.NewServeMux()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		id := e2engine.TestExecutionID(r.Context())
		if id != validID {
			t.Errorf("TestExecutionID() = %q, want %q", id, validID)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(testExecutionIDHeader, validID)
	response := httptest.NewRecorder()

	Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			response.Code,
			http.StatusNoContent,
		)
	}
}

func TestHandler_MoreThanOneHandler(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Handler() did not panic")
		}
	}()

	Handler(http.NewServeMux(), http.NewServeMux())
}

func TestInject(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		wantID      string
		wantSameReq bool
	}{
		{
			name:   "valid execution ID",
			id:     validID,
			wantID: validID,
		},
		{
			name:        "no execution ID",
			wantSameReq: true,
		},
		{
			name:        "invalid execution ID",
			id:          "execution-123",
			wantSameReq: true,
		},
		{
			name:        "uppercase execution ID",
			id:          strings.Repeat("A", 64),
			wantSameReq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)

			if tt.id != "" {
				request = request.WithContext(
					e2engine.WithTestExecutionID(
						request.Context(),
						tt.id,
					),
				)
			}

			got := Inject(request)

			if id := got.Header.Get(testExecutionIDHeader); id != tt.wantID {
				t.Fatalf("header = %q, want %q", id, tt.wantID)
			}

			if tt.wantSameReq {
				if got != request {
					t.Fatal("Inject() returned a new request without a valid execution ID")
				}
				return
			}

			if got == request {
				t.Fatal("Inject() modified the original request")
			}

			if id := request.Header.Get(testExecutionIDHeader); id != "" {
				t.Fatalf("original request header = %q, want empty", id)
			}

			got.Header.Set("X-Test", "changed")
			if request.Header.Get("X-Test") != "" {
				t.Fatal("Inject() did not clone the request headers")
			}
		})
	}
}

func TestTransport(t *testing.T) {
	var gotID string

	next := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotID = r.Header.Get(testExecutionIDHeader)
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       http.NoBody,
			Header:     make(http.Header),
			Request:    r,
		}, nil
	})

	request := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	request = request.WithContext(
		e2engine.WithTestExecutionID(context.Background(), validID),
	)

	response, err := Transport(next).RoundTrip(request)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	defer response.Body.Close()

	if gotID != validID {
		t.Fatalf("header = %q, want %q", gotID, validID)
	}

	if id := request.Header.Get(testExecutionIDHeader); id != "" {
		t.Fatalf("original request header = %q, want empty", id)
	}
}

func TestTransport_MoreThanOneTransport(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Transport() did not panic")
		}
	}()

	Transport(http.DefaultTransport, http.DefaultTransport)
}

func TestTestExecutionID(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		wantID string
	}{
		{
			name:   "valid execution ID",
			id:     validID,
			wantID: validID,
		},
		{
			name:   "raw invalid execution ID",
			id:     "execution-123",
			wantID: "execution-123",
		},
		{
			name: "no execution ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := make(http.Header)
			if tt.id != "" {
				header.Set(testExecutionIDHeader, tt.id)
			}

			if got := TestExecutionID(header); got != tt.wantID {
				t.Fatalf(
					"TestExecutionID() = %q, want %q",
					got,
					tt.wantID,
				)
			}
		})
	}
}

func TestWithTestExecutionID(t *testing.T) {
	tests := []struct {
		name       string
		existingID string
		id         string
		wantID     string
	}{
		{
			name:   "add execution ID",
			id:     validID,
			wantID: validID,
		},
		{
			name:       "replace execution ID",
			existingID: anotherID,
			id:         validID,
			wantID:     validID,
		},
		{
			name:       "empty execution ID removes existing ID",
			existingID: anotherID,
		},
		{
			name:       "invalid execution ID removes existing ID",
			existingID: anotherID,
			id:         "execution-123",
		},
		{
			name: "invalid execution ID is not added",
			id:   "execution-123",
		},
		{
			name: "uppercase execution ID is not added",
			id:   strings.Repeat("A", 64),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := make(http.Header)
			header.Set("X-Test", "value")

			if tt.existingID != "" {
				header.Set(testExecutionIDHeader, tt.existingID)
			}

			got := WithTestExecutionID(header, tt.id)

			if id := TestExecutionID(got); id != tt.wantID {
				t.Fatalf(
					"TestExecutionID() = %q, want %q",
					id,
					tt.wantID,
				)
			}

			// The original headers must remain unchanged.
			if id := TestExecutionID(header); id != tt.existingID {
				t.Fatalf(
					"original execution ID = %q, want %q",
					id,
					tt.existingID,
				)
			}

			if value := got.Get("X-Test"); value != "value" {
				t.Fatalf("X-Test = %q, want value", value)
			}

			header.Set("X-After", "source")
			if got.Get("X-After") != "" {
				t.Fatal("WithTestExecutionID() did not clone headers")
			}
		})
	}
}

func TestWithoutTestExecutionID(t *testing.T) {
	tests := []struct {
		name   string
		header http.Header
	}{
		{
			name: "execution ID",
			header: http.Header{
				"X-Test": {"value"},
			},
		},
		{
			name: "no execution ID",
			header: http.Header{
				"X-Test": {"value"},
			},
		},
		{
			name: "nil header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "execution ID" {
				tt.header.Set(testExecutionIDHeader, validID)
			}

			got := WithoutTestExecutionID(tt.header)

			if id := TestExecutionID(got); id != "" {
				t.Fatalf(
					"TestExecutionID() = %q, want empty",
					id,
				)
			}

			if tt.header != nil {
				if value := got.Get("X-Test"); value != "value" {
					t.Fatalf("X-Test = %q, want value", value)
				}

				tt.header.Set("X-After", "source")
				if got.Get("X-After") != "" {
					t.Fatal("WithoutTestExecutionID() did not clone headers")
				}
			}
		})
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
