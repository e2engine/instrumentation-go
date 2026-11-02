package grpc

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	e2engine "github.com/e2engine/instrumentation-go"
)

var (
	validID   = strings.Repeat("a1b2c3d4", 8)
	anotherID = strings.Repeat("b2c3d4e5", 8)
)

func TestExtract(t *testing.T) {
	tests := []struct {
		name        string
		values      []string
		existingID  string
		wantID      string
		wantSameCtx bool
	}{
		{
			name:   "valid execution ID",
			values: []string{validID},
			wantID: validID,
		},
		{
			name:        "no execution ID",
			wantSameCtx: true,
		},
		{
			name:        "empty incoming ID preserves existing ID",
			values:      []string{""},
			existingID:  anotherID,
			wantID:      anotherID,
			wantSameCtx: true,
		},
		{
			name:   "valid nonhex ID",
			values: []string{strings.Repeat("z", 64)},
			wantID: strings.Repeat("z", 64),
		},
		{
			name:        "invalid execution ID",
			values:      []string{"execution-123"},
			wantSameCtx: true,
		},
		{
			name:        "uppercase execution ID",
			values:      []string{strings.Repeat("A", 64)},
			wantSameCtx: true,
		},
		{
			name:        "execution ID too short",
			values:      []string{strings.Repeat("a", 63)},
			wantSameCtx: true,
		},
		{
			name:        "execution ID too long",
			values:      []string{strings.Repeat("a", 65)},
			wantSameCtx: true,
		},
		{
			name:        "multiple execution IDs",
			values:      []string{validID, anotherID},
			wantSameCtx: true,
		},
		{
			name:        "existing ID with no incoming ID",
			existingID:  anotherID,
			wantID:      anotherID,
			wantSameCtx: true,
		},
		{
			name:        "existing ID with invalid incoming ID",
			values:      []string{"invalid"},
			existingID:  anotherID,
			wantID:      anotherID,
			wantSameCtx: true,
		},
		{
			name:        "existing ID with multiple incoming IDs",
			values:      []string{validID, anotherID},
			existingID:  anotherID,
			wantID:      anotherID,
			wantSameCtx: true,
		},
		{
			name:       "valid incoming ID overrides existing ID",
			values:     []string{validID},
			existingID: anotherID,
			wantID:     validID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			if tt.existingID != "" {
				ctx = e2engine.WithTestExecutionID(ctx, tt.existingID)
			}

			if len(tt.values) > 0 {
				md := metadata.MD{
					testExecutionIDKey: tt.values,
				}
				ctx = metadata.NewIncomingContext(ctx, md)
			}

			got := Extract(ctx)

			if id := e2engine.TestExecutionID(got); id != tt.wantID {
				t.Fatalf(
					"TestExecutionID() = %q, want %q",
					id,
					tt.wantID,
				)
			}

			if tt.wantSameCtx && got != ctx {
				t.Fatal("Extract() returned a new context without a valid single execution ID")
			}

			if !tt.wantSameCtx && got == ctx {
				t.Fatal("Extract() did not create a context for a valid execution ID")
			}
		})
	}
}

func TestUnaryServerInterceptor(t *testing.T) {
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
			ctx := context.Background()
			if tt.id != "" {
				ctx = metadata.NewIncomingContext(
					ctx,
					metadata.Pairs(testExecutionIDKey, tt.id),
				)
			}

			var gotID string
			handler := func(ctx context.Context, req any) (any, error) {
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
				t.Fatalf("UnaryServerInterceptor() error = %v", err)
			}

			if gotID != tt.wantID {
				t.Fatalf("TestExecutionID() = %q, want %q", gotID, tt.wantID)
			}

			if response != "response" {
				t.Fatalf("response = %v, want response", response)
			}
		})
	}
}

func TestUnaryServerInterceptor_PreservesError(t *testing.T) {
	wantErr := errors.New("handler failed")

	handler := func(ctx context.Context, req any) (any, error) {
		return nil, wantErr
	}

	response, err := UnaryServerInterceptor()(
		context.Background(),
		"request",
		&grpc.UnaryServerInfo{},
		handler,
	)

	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if response != nil {
		t.Fatalf("response = %v, want nil", response)
	}
}

func TestInject(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		wantID      string
		wantSameCtx bool
	}{
		{
			name:   "valid execution ID",
			id:     validID,
			wantID: validID,
		},
		{
			name:        "no execution ID",
			wantSameCtx: true,
		},
		{
			name:        "invalid execution ID",
			id:          "execution-123",
			wantSameCtx: true,
		},
		{
			name:        "uppercase execution ID",
			id:          strings.Repeat("A", 64),
			wantSameCtx: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			if tt.id != "" {
				ctx = e2engine.WithTestExecutionID(ctx, tt.id)
			}

			got := Inject(ctx)

			md, _ := metadata.FromOutgoingContext(got)
			gotID := TestExecutionID(md)

			if gotID != tt.wantID {
				t.Fatalf(
					"metadata execution ID = %q, want %q",
					gotID,
					tt.wantID,
				)
			}

			if tt.wantSameCtx && got != ctx {
				t.Fatal("Inject() returned a new context without a valid execution ID")
			}

			if !tt.wantSameCtx && got == ctx {
				t.Fatal("Inject() did not create outgoing metadata for valid execution ID")
			}
		})
	}
}

func TestUnaryClientInterceptor(t *testing.T) {
	ctx := e2engine.WithTestExecutionID(
		context.Background(),
		validID,
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
		gotID = TestExecutionID(md)
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
		t.Fatalf("UnaryClientInterceptor() error = %v", err)
	}

	if gotID != validID {
		t.Fatalf("metadata execution ID = %q, want %q", gotID, validID)
	}
	if gotMethod != "/test.Service/Call" {
		t.Fatalf("method = %q, want /test.Service/Call", gotMethod)
	}
	if gotRequest != request {
		t.Fatalf("request = %v, want %v", gotRequest, request)
	}
	if gotReply != &reply {
		t.Fatalf("reply = %v, want %v", gotReply, &reply)
	}
}

func TestUnaryClientInterceptor_PreservesError(t *testing.T) {
	wantErr := errors.New("invocation failed")

	invoker := func(
		ctx context.Context,
		method string,
		req any,
		reply any,
		cc *grpc.ClientConn,
		opts ...grpc.CallOption,
	) error {
		return wantErr
	}

	err := UnaryClientInterceptor()(
		context.Background(),
		"/test.Service/Call",
		"request",
		nil,
		nil,
		invoker,
	)

	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestTestExecutionID(t *testing.T) {
	tests := []struct {
		name   string
		md     metadata.MD
		wantID string
	}{
		{
			name:   "valid execution ID",
			md:     metadata.Pairs(testExecutionIDKey, validID),
			wantID: validID,
		},
		{
			name:   "raw invalid execution ID",
			md:     metadata.Pairs(testExecutionIDKey, "execution-123"),
			wantID: "execution-123",
		},
		{
			name: "no execution ID",
			md:   metadata.Pairs("x-test", "value"),
		},
		{
			name: "nil metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TestExecutionID(tt.md); got != tt.wantID {
				t.Fatalf("TestExecutionID() = %q, want %q", got, tt.wantID)
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
			name:       "empty ID removes existing ID",
			existingID: anotherID,
		},
		{
			name:       "invalid ID removes existing ID",
			existingID: anotherID,
			id:         "execution-123",
		},
		{
			name: "invalid ID is not added",
			id:   "execution-123",
		},
		{
			name: "uppercase ID is not added",
			id:   strings.Repeat("A", 64),
		},
		{
			name:   "nil metadata",
			id:     validID,
			wantID: validID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var md metadata.MD
			if tt.name != "nil metadata" {
				md = metadata.Pairs("x-test", "value")
			}
			if tt.existingID != "" {
				md.Set(testExecutionIDKey, tt.existingID)
			}

			got := WithTestExecutionID(md, tt.id)

			if id := TestExecutionID(got); id != tt.wantID {
				t.Fatalf("TestExecutionID() = %q, want %q", id, tt.wantID)
			}

			if id := TestExecutionID(md); id != tt.existingID {
				t.Fatalf("original metadata ID = %q, want %q", id, tt.existingID)
			}

			if md != nil {
				if value := got.Get("x-test"); len(value) != 1 || value[0] != "value" {
					t.Fatalf("x-test = %v, want [value]", value)
				}

				md.Set("x-after", "source")
				if len(got.Get("x-after")) != 0 {
					t.Fatal("WithTestExecutionID() did not copy metadata")
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
				testExecutionIDKey, validID,
				"x-test", "value",
			),
		},
		{
			name: "no execution ID",
			md:   metadata.Pairs("x-test", "value"),
		},
		{
			name: "nil metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WithoutTestExecutionID(tt.md)

			if id := TestExecutionID(got); id != "" {
				t.Fatalf("TestExecutionID() = %q, want empty", id)
			}

			if tt.md != nil {
				if values := got.Get("x-test"); len(values) != 1 ||
					values[0] != "value" {
					t.Fatalf("x-test = %v, want [value]", values)
				}

				tt.md.Set("x-after", "source")
				if len(got.Get("x-after")) != 0 {
					t.Fatal("WithoutTestExecutionID() did not copy metadata")
				}
			}
		})
	}
}

func TestInject_ExistingExecutionIDTakesPrecedence(t *testing.T) {
	ctx := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs(
			testExecutionIDKey, anotherID,
			"x-test", "value",
		),
	)

	ctx = e2engine.WithTestExecutionID(ctx, validID)

	got := Inject(ctx)
	if got != ctx {
		t.Fatal("Inject() replaced a context with a valid outgoing execution ID")
	}

	md, ok := metadata.FromOutgoingContext(got)
	if !ok {
		t.Fatal("Inject() did not produce outgoing metadata")
	}

	values := md.Get(testExecutionIDKey)
	if len(values) != 1 || values[0] != anotherID {
		t.Fatalf(
			"execution IDs = %v, want [%s]",
			values,
			anotherID,
		)
	}

	if values := md.Get("x-test"); len(values) != 1 ||
		values[0] != "value" {
		t.Fatalf("x-test = %v, want [value]", values)
	}

	// Verify the original outgoing metadata remains unchanged.
	original, _ := metadata.FromOutgoingContext(ctx)
	if id := TestExecutionID(original); id != anotherID {
		t.Fatalf(
			"original execution ID = %q, want %q",
			id,
			anotherID,
		)
	}
}

func TestOutgoingPropagation(t *testing.T) {
	tests := []struct {
		name       string
		values     []string
		exists     bool
		appended   []string
		explicitID string
	}{
		{name: "missing"},
		{name: "nil values", exists: true},
		{name: "empty values", exists: true, values: []string{}},
		{name: "empty ID", exists: true, values: []string{""}},
		{name: "valid ID", exists: true, values: []string{validID}, explicitID: validID},
		{
			name: "valid nonhex ID", exists: true,
			values: []string{strings.Repeat("z", 64)}, explicitID: strings.Repeat("z", 64),
		},
		{name: "malformed ID", exists: true, values: []string{"execution-123"}},
		{name: "uppercase ID", exists: true, values: []string{strings.Repeat("A", 64)}},
		{name: "short ID", exists: true, values: []string{strings.Repeat("a", 63)}},
		{name: "long ID", exists: true, values: []string{strings.Repeat("a", 65)}},
		{name: "multiple IDs", exists: true, values: []string{validID, anotherID}},
		{name: "duplicate IDs", exists: true, values: []string{validID, validID}},
		{name: "mixed valid and invalid IDs", exists: true, values: []string{validID, "invalid"}},
		{name: "comma joined IDs", exists: true, values: []string{validID + "," + anotherID}},
		{name: "CRLF", exists: true, values: []string{validID + "\r\nx-test: injected"}},
		{name: "appended valid ID", appended: []string{validID}, explicitID: validID},
		{name: "appended invalid ID", appended: []string{"invalid"}},
		{
			name: "ID in base and appended metadata", exists: true,
			values: []string{validID}, appended: []string{anotherID},
		},
		{name: "multiple appended IDs", appended: []string{validID, anotherID}},
	}

	for _, tt := range tests {
		for _, contextID := range []string{"", anotherID} {
			contextName := "without context ID"
			if contextID != "" {
				contextName = "with context ID"
			}
			for _, intercept := range []bool{false, true} {
				mode := "Inject"
				if intercept {
					mode = "UnaryClientInterceptor"
				}
				t.Run(mode+"/"+tt.name+"/"+contextName, func(t *testing.T) {
					ctx := e2engine.WithTestExecutionID(context.Background(), contextID)
					incoming := metadata.Pairs("incoming-only", "preserved")
					ctx = metadata.NewIncomingContext(ctx, incoming)
					md := metadata.Pairs("x-test", "one", "x-test", "two")
					if tt.exists {
						md[testExecutionIDKey] = tt.values
					}
					originalMD := make(metadata.MD, len(md))
					for key, values := range md {
						originalMD[key] = slices.Clone(values)
					}
					ctx = metadata.NewOutgoingContext(ctx, md)
					ctx = metadata.AppendToOutgoingContext(ctx, "x-appended", "preserved")
					for _, id := range tt.appended {
						ctx = metadata.AppendToOutgoingContext(ctx, testExecutionIDKey, id)
					}
					originalOutgoing, _ := metadata.FromOutgoingContext(ctx)
					wantID := contextID
					if tt.explicitID != "" {
						wantID = tt.explicitID
					}
					wantSame := tt.explicitID != "" || (!tt.exists && len(tt.appended) == 0 && contextID == "")

					var got context.Context
					if intercept {
						wantErr := errors.New("invocation failed")
						request, reply := new(int), new(int)
						connection := new(grpc.ClientConn)
						option := grpc.WaitForReady(true)
						calls := 0
						invoker := func(callCtx context.Context, method string, req, resp any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
							calls++
							got = callCtx
							if method != "/test.Service/Call" || req != request || resp != reply || cc != connection ||
								len(opts) != 1 || opts[0] != option {
								t.Error("interceptor changed invocation arguments")
							}
							return wantErr
						}
						err := UnaryClientInterceptor()(ctx, "/test.Service/Call", request, reply, connection, invoker, option)
						if err != wantErr || calls != 1 {
							t.Fatalf("error = %v, calls = %d; want %v, 1 call", err, calls, wantErr)
						}
					} else {
						got = Inject(ctx)
					}

					outgoing, _ := metadata.FromOutgoingContext(got)
					want := metadata.Pairs("x-test", "one", "x-test", "two", "x-appended", "preserved")
					if wantID != "" {
						want.Set(testExecutionIDKey, wantID)
					}
					if !reflect.DeepEqual(outgoing, want) {
						t.Fatalf("outgoing metadata = %#v, want %#v", outgoing, want)
					}
					if (got == ctx) != wantSame {
						t.Fatalf("original context returned = %v, want %v", got == ctx, wantSame)
					}
					if Inject(got) != got {
						t.Fatal("repeated injection changed normalized metadata")
					}
					unchanged, _ := metadata.FromOutgoingContext(ctx)
					if !reflect.DeepEqual(unchanged, originalOutgoing) || !reflect.DeepEqual(md, originalMD) {
						t.Fatal("original outgoing metadata changed")
					}
					gotIncoming, _ := metadata.FromIncomingContext(got)
					if !reflect.DeepEqual(gotIncoming, incoming) || e2engine.TestExecutionID(got) != contextID {
						t.Fatal("incoming metadata or context execution ID changed")
					}
				})
			}
		}
	}
}
