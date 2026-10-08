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

	e2engine "github.com/e2engine/instrumentation-go"
	e2enginehttp "github.com/e2engine/instrumentation-go/http"
)

func TestOption(t *testing.T) {
	tests := []struct {
		name                        string
		ctx                         context.Context
		wantExecutionID             string
		wantIncludedInSignedHeaders bool
	}{
		{
			name: "propagates test execution id",
			ctx: e2engine.WithTestExecutionID(
				context.Background(),
				"execution-123",
			),
			wantExecutionID:             "execution-123",
			wantIncludedInSignedHeaders: true,
		},
		{
			name:                        "no test execution id",
			ctx:                         context.Background(),
			wantIncludedInSignedHeaders: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request *http.Request

			server := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					request = r.Clone(r.Context())

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

			if request == nil {
				t.Fatal("expected request")
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
