package aws

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	e2engine "github.com/e2engine/instrumentation-go"
	e2enginehttp "github.com/e2engine/instrumentation-go/http"
)

func TestOption(t *testing.T) {
	tests := []struct {
		name                        string
		ctx                         context.Context
		headers                     []string
		wantExecutionID             string
		wantIncludedInSignedHeaders bool
	}{
		{
			name: "propagates test execution id",
			ctx: e2engine.WithTestExecutionID(
				context.Background(),
				validID,
			),
			wantExecutionID:             validID,
			wantIncludedInSignedHeaders: true,
		},
		{
			name:                        "no test execution id",
			ctx:                         context.Background(),
			wantIncludedInSignedHeaders: false,
		},
		{
			name:            "explicit ID overrides context",
			ctx:             e2engine.WithTestExecutionID(context.Background(), anotherID),
			headers:         []string{validID},
			wantExecutionID: validID, wantIncludedInSignedHeaders: true,
		},
		{
			name: "explicit ID without context",
			ctx:  context.Background(), headers: []string{validID},
			wantExecutionID: validID, wantIncludedInSignedHeaders: true,
		},
		{
			name:            "invalid ID replaced before signing",
			ctx:             e2engine.WithTestExecutionID(context.Background(), anotherID),
			headers:         []string{"invalid"},
			wantExecutionID: anotherID, wantIncludedInSignedHeaders: true,
		},
		{
			name:            "multiple IDs replaced before signing",
			ctx:             e2engine.WithTestExecutionID(context.Background(), anotherID),
			headers:         []string{validID, anotherID},
			wantExecutionID: anotherID, wantIncludedInSignedHeaders: true,
		},
		{
			name: "invalid ID removed before signing",
			ctx:  context.Background(), headers: []string{"invalid"},
		},
		{
			name: "multiple IDs removed before signing",
			ctx:  context.Background(), headers: []string{validID, anotherID},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := make(chan *http.Request, 1)

			server := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					requests <- r.Clone(r.Context())

					_, _ = io.Copy(io.Discard, r.Body)

					w.Header().Set(
						"Content-Type",
						"application/x-amz-json-1.0",
					)
					_, _ = w.Write([]byte(`{}`))
				},
			))
			defer server.Close()

			cfg, err := config.LoadDefaultConfig(
				context.Background(),
				config.WithRegion("eu-west-1"),
				config.WithCredentialsProvider(
					credentials.NewStaticCredentialsProvider(
						"local",
						"local",
						"",
					),
				),
				Option(),
				config.WithAPIOptions([]func(*smithymiddleware.Stack) error{
					func(stack *smithymiddleware.Stack) error {
						return stack.Build.Insert(smithymiddleware.BuildMiddlewareFunc(
							"ExplicitExecutionID",
							func(ctx context.Context, in smithymiddleware.BuildInput, next smithymiddleware.BuildHandler) (smithymiddleware.BuildOutput, smithymiddleware.Metadata, error) {
								request := in.Request.(*smithyhttp.Request)
								for _, value := range tt.headers {
									request.Header.Add(testExecutionIDHeader, value)
								}
								return next.HandleBuild(ctx, in)
							},
						), "E2EnginePropagation", smithymiddleware.Before)
					},
				}),
			)
			if err != nil {
				t.Fatalf("load AWS config: %v", err)
			}

			client := dynamodb.NewFromConfig(
				cfg,
				func(options *dynamodb.Options) {
					options.BaseEndpoint = aws.String(server.URL)
				},
			)

			_, err = client.ListTables(
				tt.ctx,
				&dynamodb.ListTablesInput{},
			)
			if err != nil {
				t.Fatalf("ListTables: %v", err)
			}

			var request *http.Request
			select {
			case request = <-requests:
			default:
				t.Fatal("expected request")
			}
			values := request.Header.Values(testExecutionIDHeader)
			if tt.wantExecutionID != "" && len(values) != 1 {
				t.Fatalf("execution ID values = %q, want exactly one", values)
			}
			if tt.wantExecutionID == "" {
				if _, exists := request.Header[http.CanonicalHeaderKey(testExecutionIDHeader)]; exists {
					t.Fatal("execution ID header was not removed")
				}
			}

			if got := e2enginehttp.TestExecutionID(request.Header); got != tt.wantExecutionID {
				t.Fatalf(
					"execution ID header = %q, want %q",
					got,
					tt.wantExecutionID,
				)
			}

			authorization := strings.ToLower(
				request.Header.Get("Authorization"),
			)
			if authorization == "" {
				t.Fatal("expected Authorization header")
			}

			includedInSignedHeaders := strings.Contains(
				authorization,
				strings.ToLower(
					"e2engine-test-execution-id",
				),
			)

			if includedInSignedHeaders != tt.wantIncludedInSignedHeaders {
				t.Fatalf(
					"E2Engine header inclusion in AWS SignedHeaders = %v, want %v: %q",
					includedInSignedHeaders,
					tt.wantIncludedInSignedHeaders,
					authorization,
				)
			}
		})
	}
}
