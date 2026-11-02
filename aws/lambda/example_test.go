package lambda_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	e2engine "github.com/e2engine/instrumentation-go"
	e2enginelambda "github.com/e2engine/instrumentation-go/aws/lambda"
)

func ExampleHandler() {
	handle := func(ctx context.Context, _ events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		// Pass ctx to instrumented HTTP, gRPC, or AWS SDK dependency calls.
		return events.APIGatewayV2HTTPResponse{StatusCode: http.StatusOK}, nil
	}

	// Wrap an API Gateway HTTP API payload format 2.0 handler.
	lambda.Start(e2enginelambda.Handler(handle))
}

func ExampleExtract() {
	request := events.APIGatewayV2HTTPRequest{
		Headers: map[string]string{
			"e2engine-test-execution-id": strings.Repeat("a", 64),
		},
	}
	ctx := e2enginelambda.Extract(context.Background(), request)
	fmt.Println(e2engine.TestExecutionID(ctx))
	// Output: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
}

func ExampleTestExecutionID() {
	// The raw getter does not validate; Extract rejects this value.
	headers := map[string]string{"e2engine-test-execution-id": "invalid"}
	fmt.Println(e2enginelambda.TestExecutionID(headers))
	// Output: invalid
}
