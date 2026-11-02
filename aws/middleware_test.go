package aws

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	e2engine "github.com/e2engine/instrumentation-go"
)

const testExecutionIDHeader = "E2Engine-Test-Execution-ID"

var (
	validID   = strings.Repeat("a1b2c3d4", 8)
	anotherID = strings.Repeat("b2c3d4e5", 8)
)

func TestMiddleware_OutgoingPropagation(t *testing.T) {
	tests := []struct {
		name       string
		values     []string
		exists     bool
		nilHeader  bool
		explicitID string
	}{
		{name: "missing"},
		{name: "nil header", nilHeader: true},
		{name: "nil values", exists: true},
		{name: "empty values", exists: true, values: []string{}},
		{name: "empty ID", exists: true, values: []string{""}},
		{name: "valid ID", exists: true, values: []string{validID}, explicitID: validID},
		{name: "nonhex ID", exists: true, values: []string{strings.Repeat("z", 64)}, explicitID: strings.Repeat("z", 64)},
		{name: "malformed ID", exists: true, values: []string{"execution-123"}},
		{name: "uppercase ID", exists: true, values: []string{strings.Repeat("A", 64)}},
		{name: "short ID", exists: true, values: []string{strings.Repeat("a", 63)}},
		{name: "long ID", exists: true, values: []string{strings.Repeat("a", 65)}},
		{name: "multiple IDs", exists: true, values: []string{validID, anotherID}},
		{name: "duplicate IDs", exists: true, values: []string{validID, validID}},
		{name: "mixed IDs", exists: true, values: []string{validID, "invalid"}},
		{name: "comma joined IDs", exists: true, values: []string{validID + "," + anotherID}},
		{name: "CRLF", exists: true, values: []string{validID + "\r\nX-Test: injected"}},
	}

	for _, tt := range tests {
		for _, contextID := range []string{"", anotherID} {
			contextName := "without context ID"
			if contextID != "" {
				contextName = "with context ID"
			}
			t.Run(tt.name+"/"+contextName, func(t *testing.T) {
				ctx := e2engine.WithTestExecutionID(context.Background(), contextID)
				request := smithyhttp.NewStackRequest().(*smithyhttp.Request)
				stream := strings.NewReader("request body")
				var err error
				request, err = request.SetStream(stream)
				if err != nil {
					t.Fatal(err)
				}
				// A different HTTP context must not supply the fallback ID.
				httpContext := e2engine.WithTestExecutionID(context.Background(), validID)
				request.Request = request.Request.WithContext(httpContext)
				if tt.nilHeader {
					request.Header = nil
				} else {
					request.Header.Set("X-Test", "preserved")
					if tt.exists {
						request.Header[http.CanonicalHeaderKey(testExecutionIDHeader)] = tt.values
					}
				}
				originalHTTP := request.Request
				originalHeaders := request.Header.Clone()
				wantID := contextID
				if tt.explicitID != "" {
					wantID = tt.explicitID
				}
				wantSame := tt.explicitID != "" || (!tt.exists && contextID == "")
				wantErr := errors.New("next middleware failed")
				result := new(int)
				var wantMetadata smithymiddleware.Metadata
				wantMetadata.Set("test", result)
				calls := 0
				next := smithymiddleware.BuildHandlerFunc(func(gotCtx context.Context, in smithymiddleware.BuildInput) (smithymiddleware.BuildOutput, smithymiddleware.Metadata, error) {
					calls++
					got := in.Request.(*smithyhttp.Request)
					if gotCtx != ctx || got.Context() != httpContext || got.GetStream() != stream {
						t.Error("operation context, HTTP context, or stream changed")
					}
					if (got == request) != wantSame {
						t.Errorf("original request returned = %v, want %v", got == request, wantSame)
					}
					wantHeaders := originalHeaders.Clone()
					if wantID != "" {
						if wantHeaders == nil {
							wantHeaders = make(http.Header)
						}
						wantHeaders.Set(testExecutionIDHeader, wantID)
					} else {
						wantHeaders.Del(testExecutionIDHeader)
					}
					if !reflect.DeepEqual(got.Header, wantHeaders) {
						t.Errorf("headers = %#v, want %#v", got.Header, wantHeaders)
					}
					if !wantSame {
						got.Header.Set("X-Test", "changed")
					}
					return smithymiddleware.BuildOutput{Result: result}, wantMetadata, wantErr
				})
				out, md, err := (middleware{}).HandleBuild(ctx, smithymiddleware.BuildInput{Request: request}, next)
				if err != wantErr || out.Result != result || md.Get("test") != result || calls != 1 {
					t.Fatalf("middleware did not preserve output, metadata, error, or call count: %v", err)
				}
				if request.Request != originalHTTP || !reflect.DeepEqual(request.Header, originalHeaders) {
					t.Fatal("original request or headers changed")
				}
			})
		}
	}
}

func TestMiddleware_UnsupportedRequest(t *testing.T) {
	for _, request := range []any{struct{}{}, (*smithyhttp.Request)(nil), &smithyhttp.Request{}} {
		for _, id := range []string{"", validID} {
			ctx := e2engine.WithTestExecutionID(context.Background(), id)
			called := false
			next := smithymiddleware.BuildHandlerFunc(func(gotCtx context.Context, in smithymiddleware.BuildInput) (smithymiddleware.BuildOutput, smithymiddleware.Metadata, error) {
				called = true
				if gotCtx != ctx || in.Request != request {
					t.Error("unsupported request or context changed")
				}
				return smithymiddleware.BuildOutput{}, smithymiddleware.Metadata{}, nil
			})
			_, _, err := (middleware{}).HandleBuild(ctx, smithymiddleware.BuildInput{Request: request}, next)
			if (err != nil) != (id != "") || called != (id == "") {
				t.Fatalf("request %T, ID %q: error = %v, called = %v", request, id, err, called)
			}
		}
	}
}
