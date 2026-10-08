package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	e2engine "github.com/e2engine/instrumentation-go"
)

func TestExtract(t *testing.T) {
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
			ctx := context.Background()
			if tt.id != "" {
				ctx = metadata.NewIncomingContext(
					ctx,
					metadata.Pairs(
						testExecutionIDKey,
						tt.id,
					),
				)
			}

			got := Extract(ctx)

			if id := e2engine.TestExecutionID(got); id != tt.wantID {
				t.Fatalf(
					"TestExecutionID() = %q, want %q",
					id,
					tt.wantID,
				)
			}

			if tt.id == "" && got != ctx {
				t.Fatal(
					"Extract() returned a new context without an execution ID",
				)
			}
		})
	}
}

func TestUnaryServerInterceptor(t *testing.T) {
	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs(
			testExecutionIDKey,
			"execution-123",
		),
	)

	var gotID string

	handler := func(
		ctx context.Context,
		req any,
	) (any, error) {
		gotID = e2engine.TestExecutionID(ctx)
		return "response", nil
	}

	response, err := UnaryServerInterceptor()(
		ctx,
		"request",
		&grpc.UnaryServerInfo{},
		handler,
	)
	if err != nil {
		t.Fatalf(
			"UnaryServerInterceptor() error = %v",
			err,
		)
	}

	if gotID != "execution-123" {
		t.Fatalf(
			"TestExecutionID() = %q, want %q",
			gotID,
			"execution-123",
		)
	}

	if response != "response" {
		t.Fatalf(
			"response = %v, want %q",
			response,
			"response",
		)
	}
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
			ctx := context.Background()
			if tt.id != "" {
				ctx = e2engine.WithTestExecutionID(
					ctx,
					tt.id,
				)
			}

			got := Inject(ctx)

			md, _ := metadata.FromOutgoingContext(got)
			values := md.Get(testExecutionIDKey)

			var gotID string
			if len(values) > 0 {
				gotID = values[0]
			}

			if gotID != tt.wantID {
				t.Fatalf(
					"metadata execution ID = %q, want %q",
					gotID,
					tt.wantID,
				)
			}

			if tt.id == "" && got != ctx {
				t.Fatal(
					"Inject() returned a new context without an execution ID",
				)
			}
		})
	}
}

func TestUnaryClientInterceptor(t *testing.T) {
	ctx := e2engine.WithTestExecutionID(
		context.Background(),
		"execution-123",
	)

	var gotID string
	var gotMethod string
	var gotRequest any
	var gotReply any

	invoker := func(
		ctx context.Context,
		method string,
		req any,
		reply any,
		cc *grpc.ClientConn,
		opts ...grpc.CallOption,
	) error {
		md, _ := metadata.FromOutgoingContext(ctx)

		values := md.Get(testExecutionIDKey)
		if len(values) > 0 {
			gotID = values[0]
		}

		gotMethod = method
		gotRequest = req
		gotReply = reply

		return nil
	}

	request := "request"
	reply := "reply"

	err := UnaryClientInterceptor()(
		ctx,
		"/test.Service/Call",
		request,
		&reply,
		nil,
		invoker,
	)
	if err != nil {
		t.Fatalf(
			"UnaryClientInterceptor() error = %v",
			err,
		)
	}

	if gotID != "execution-123" {
		t.Fatalf(
			"metadata execution ID = %q, want %q",
			gotID,
			"execution-123",
		)
	}

	if gotMethod != "/test.Service/Call" {
		t.Fatalf(
			"method = %q, want %q",
			gotMethod,
			"/test.Service/Call",
		)
	}

	if gotRequest != request {
		t.Fatalf(
			"request = %v, want %v",
			gotRequest,
			request,
		)
	}

	if gotReply != &reply {
		t.Fatalf(
			"reply = %v, want %v",
			gotReply,
			&reply,
		)
	}
}

func TestTestExecutionID(t *testing.T) {
	tests := []struct {
		name   string
		md     metadata.MD
		wantID string
	}{
		{
			name: "execution ID",
			md: metadata.Pairs(
				testExecutionIDKey,
				"execution-123",
			),
			wantID: "execution-123",
		},
		{
			name: "no execution ID",
			md: metadata.Pairs(
				"x-test",
				"value",
			),
		},
		{
			name: "nil metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TestExecutionID(tt.md); got != tt.wantID {
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
		name   string
		md     metadata.MD
		id     string
		wantID string
	}{
		{
			name: "add execution ID",
			md: metadata.Pairs(
				"x-test",
				"value",
			),
			id:     "execution-123",
			wantID: "execution-123",
		},
		{
			name: "replace execution ID",
			md: metadata.Pairs(
				testExecutionIDKey,
				"execution-old",
			),
			id:     "execution-123",
			wantID: "execution-123",
		},
		{
			name: "empty execution ID removes existing ID",
			md: metadata.Pairs(
				testExecutionIDKey,
				"execution-old",
			),
		},
		{
			name:   "nil metadata",
			id:     "execution-123",
			wantID: "execution-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WithTestExecutionID(tt.md, tt.id)

			if id := TestExecutionID(got); id != tt.wantID {
				t.Fatalf(
					"TestExecutionID() = %q, want %q",
					id,
					tt.wantID,
				)
			}

			if values := got.Get("x-test"); len(values) > 0 &&
				values[0] != "value" {
				t.Fatalf(
					"x-test = %q, want %q",
					values[0],
					"value",
				)
			}

			if tt.md != nil {
				tt.md.Set("x-after", "source")

				if len(got.Get("x-after")) != 0 {
					t.Fatal(
						"WithTestExecutionID() did not copy metadata",
					)
				}
			}
		})
	}
}

func TestWithoutTestExecutionID(t *testing.T) {
	tests := []struct {
		name string
		md   metadata.MD
	}{
		{
			name: "execution ID",
			md: metadata.Pairs(
				testExecutionIDKey,
				"execution-123",
				"x-test",
				"value",
			),
		},
		{
			name: "no execution ID",
			md: metadata.Pairs(
				"x-test",
				"value",
			),
		},
		{
			name: "nil metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WithoutTestExecutionID(tt.md)

			if id := TestExecutionID(got); id != "" {
				t.Fatalf(
					"TestExecutionID() = %q, want empty",
					id,
				)
			}

			if values := got.Get("x-test"); len(values) > 0 &&
				values[0] != "value" {
				t.Fatalf(
					"x-test = %q, want %q",
					values[0],
					"value",
				)
			}

			if tt.md != nil {
				tt.md.Set("x-after", "source")

				if len(got.Get("x-after")) != 0 {
					t.Fatal(
						"WithoutTestExecutionID() did not copy metadata",
					)
				}
			}
		})
	}
}
