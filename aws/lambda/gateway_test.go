package lambda

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	e2engine "github.com/e2engine/instrumentation-go"
)

var (
	validID   = strings.Repeat("a1b2c3d4", 8)
	anotherID = strings.Repeat("b2c3d4e5", 8)
)

func TestIncomingPropagation(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		wantRaw string
		wantID  string
	}{
		{name: "missing"},
		{name: "unrelated header", headers: map[string]string{"x-other": "preserved"}},
		{name: "empty", headers: map[string]string{testExecutionIDHeader: ""}},
		{name: "lowercase header", headers: map[string]string{testExecutionIDHeader: validID}, wantRaw: validID, wantID: validID},
		{name: "canonical header", headers: map[string]string{"E2Engine-Test-Execution-ID": validID}, wantRaw: validID, wantID: validID},
		{name: "mixed case header", headers: map[string]string{"E2eNgInE-tEsT-eXeCuTiOn-Id": validID}, wantRaw: validID, wantID: validID},
		{
			name: "nonhex ID", headers: map[string]string{testExecutionIDHeader: strings.Repeat("z", 64)},
			wantRaw: strings.Repeat("z", 64), wantID: strings.Repeat("z", 64),
		},
		{name: "malformed", headers: map[string]string{testExecutionIDHeader: "execution-123"}, wantRaw: "execution-123"},
		{name: "uppercase", headers: map[string]string{testExecutionIDHeader: strings.Repeat("A", 64)}, wantRaw: strings.Repeat("A", 64)},
		{name: "short", headers: map[string]string{testExecutionIDHeader: strings.Repeat("a", 63)}, wantRaw: strings.Repeat("a", 63)},
		{name: "long", headers: map[string]string{testExecutionIDHeader: strings.Repeat("a", 65)}, wantRaw: strings.Repeat("a", 65)},
		{name: "whitespace", headers: map[string]string{testExecutionIDHeader: " " + validID}, wantRaw: " " + validID},
		{name: "CRLF", headers: map[string]string{testExecutionIDHeader: validID + "\r\nx-test: injected"}, wantRaw: validID + "\r\nx-test: injected"},
		{
			name: "comma joined IDs", headers: map[string]string{testExecutionIDHeader: validID + "," + anotherID},
			wantRaw: validID + "," + anotherID,
		},
		{
			name: "duplicate joined IDs", headers: map[string]string{testExecutionIDHeader: validID + "," + validID},
			wantRaw: validID + "," + validID,
		},
		{
			name:    "duplicate header casing",
			headers: map[string]string{testExecutionIDHeader: validID, "E2Engine-Test-Execution-ID": anotherID},
		},
		{
			name:    "duplicate casing with same ID",
			headers: map[string]string{testExecutionIDHeader: validID, "E2Engine-Test-Execution-ID": validID},
		},
		{
			name:    "duplicate casing with empty ID",
			headers: map[string]string{testExecutionIDHeader: validID, "E2Engine-Test-Execution-ID": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TestExecutionID(tt.headers); got != tt.wantRaw {
				t.Fatalf("raw ID = %q, want %q", got, tt.wantRaw)
			}
			for _, existingID := range []string{"", anotherID} {
				for _, wrap := range []bool{false, true} {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					ctx = e2engine.WithTestExecutionID(ctx, existingID)
					request := events.APIGatewayV2HTTPRequest{
						Version: "2.0", RawPath: "/test", Body: "request body",
						Headers: maps.Clone(tt.headers), Cookies: []string{"session=test"},
						QueryStringParameters: map[string]string{"query": "value"},
					}
					original := request
					original.Headers = maps.Clone(request.Headers)
					original.Cookies = slices.Clone(request.Cookies)
					original.QueryStringParameters = maps.Clone(request.QueryStringParameters)
					var got context.Context
					if wrap {
						wantResponse := events.APIGatewayV2HTTPResponse{
							StatusCode: 201, Body: "response body", IsBase64Encoded: true,
							Headers: map[string]string{"x-response": "preserved"}, Cookies: []string{"response=test"},
						}
						wantErr := errors.New("handler failed")
						calls := 0
						next := func(callCtx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
							calls++
							got = callCtx
							if !reflect.DeepEqual(event, original) {
								t.Error("handler received a changed request")
							}
							return wantResponse, wantErr
						}
						response, err := Handler(next)(ctx, request)
						if err != wantErr || !reflect.DeepEqual(response, wantResponse) || calls != 1 {
							t.Fatal("wrapper changed the response, error, or handler call count")
						}
					} else {
						got = Extract(ctx, request)
					}
					wantID := tt.wantID
					if wantID == "" {
						wantID = existingID
					}
					if id := e2engine.TestExecutionID(got); id != wantID {
						t.Fatalf("context ID = %q, want %q (wrapped=%v)", id, wantID, wrap)
					}
					if (got == ctx) != (tt.wantID == "") {
						t.Fatal("unexpected context identity")
					}
					if e2engine.TestExecutionID(ctx) != existingID || !reflect.DeepEqual(request, original) {
						t.Fatal("original context or request changed")
					}
					cancel()
					if got.Done() != ctx.Done() || got.Err() != context.Canceled {
						t.Fatal("context cancellation was not preserved")
					}
				}
			}
		})
	}
}

func TestHandler_Success(t *testing.T) {
	want := events.APIGatewayV2HTTPResponse{StatusCode: 204}
	next := func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		return want, nil
	}
	got, err := Handler(next)(context.Background(), events.APIGatewayV2HTTPRequest{})
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Handler() = (%#v, %v), want (%#v, nil)", got, err, want)
	}
}
