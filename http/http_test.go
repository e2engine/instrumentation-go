package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	e2engine "github.com/e2engine/instrumentation-go"
)

func TestExtract(t *testing.T) {
	tests := []struct {
		name   string
		header string
		wantID string
	}{
		{
			name:   "execution ID",
			header: "execution-123",
			wantID: "execution-123",
		},
		{
			name: "no execution ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				request.Header.Set(testExecutionIDHeader, tt.header)
			}

			got := Extract(request)

			if id := e2engine.TestExecutionID(got.Context()); id != tt.wantID {
				t.Fatalf("TestExecutionID() = %q, want %q", id, tt.wantID)
			}

			if tt.header == "" && got != request {
				t.Fatal("Extract() returned a new request without an execution ID")
			}
		})
	}
}

func TestHandler(t *testing.T) {
	var gotID string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = e2engine.TestExecutionID(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(testExecutionIDHeader, "execution-123")
	response := httptest.NewRecorder()

	Handler(next).ServeHTTP(response, request)

	if gotID != "execution-123" {
		t.Fatalf("TestExecutionID() = %q, want %q", gotID, "execution-123")
	}
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
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
		if id != "execution-123" {
			t.Fatalf("TestExecutionID() = %q, want %q", id, "execution-123")
		}

		w.WriteHeader(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(testExecutionIDHeader, "execution-123")
	response := httptest.NewRecorder()

	Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
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
		name   string
		id     string
		wantID string
	}{
		{
			name:   "execution ID",
			id:     "execution-123",
			wantID: "execution-123",
		},
		{
			name: "no execution ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.id != "" {
				request = request.WithContext(
					e2engine.WithTestExecutionID(request.Context(), tt.id),
				)
			}

			got := Inject(request)

			if id := got.Header.Get(testExecutionIDHeader); id != tt.wantID {
				t.Fatalf("header = %q, want %q", id, tt.wantID)
			}

			if tt.id == "" {
				if got != request {
					t.Fatal("Inject() returned a new request without an execution ID")
				}
				return
			}

			if got == request {
				t.Fatal("Inject() modified the original request")
			}
			if id := request.Header.Get(testExecutionIDHeader); id != "" {
				t.Fatalf("original request header = %q, want empty", id)
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
		e2engine.WithTestExecutionID(context.Background(), "execution-123"),
	)

	response, err := Transport(next).RoundTrip(request)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	defer response.Body.Close()

	if gotID != "execution-123" {
		t.Fatalf("header = %q, want %q", gotID, "execution-123")
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
			name:   "execution ID",
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
			id:     "execution-123",
			wantID: "execution-123",
		},
		{
			name:       "replace execution ID",
			existingID: "execution-old",
			id:         "execution-123",
			wantID:     "execution-123",
		},
		{
			name:       "empty execution ID removes existing ID",
			existingID: "execution-old",
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

			if value := got.Get("X-Test"); value != "value" {
				t.Fatalf(
					"X-Test = %q, want %q",
					value,
					"value",
				)
			}

			header.Set("X-After", "source")
			if got.Get("X-After") != "" {
				t.Fatal(
					"WithTestExecutionID() did not clone header",
				)
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
				testExecutionIDHeader: []string{"execution-123"},
				"X-Test":              []string{"value"},
			},
		},
		{
			name: "no execution ID",
			header: http.Header{
				"X-Test": []string{"value"},
			},
		},
		{
			name: "nil header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WithoutTestExecutionID(tt.header)

			if id := TestExecutionID(got); id != "" {
				t.Fatalf(
					"TestExecutionID() = %q, want empty",
					id,
				)
			}

			if value := got.Get("X-Test"); value != "" &&
				value != "value" {
				t.Fatalf(
					"X-Test = %q, want %q",
					value,
					"value",
				)
			}

			if tt.header != nil {
				tt.header.Set("X-After", "source")

				if got.Get("X-After") != "" {
					t.Fatal(
						"WithoutTestExecutionID() did not clone header",
					)
				}
			}
		})
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
