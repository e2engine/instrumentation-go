package lambda

import (
	"context"
	"strings"

	"github.com/aws/aws-lambda-go/events"

	e2engine "github.com/e2engine/instrumentation-go"
)

const testExecutionIDHeader = "e2engine-test-execution-id"

// APIGatewayV2Handler handles an API Gateway HTTP API payload format 2.0 event.
type APIGatewayV2Handler func(
	ctx context.Context,
	request events.APIGatewayV2HTTPRequest,
) (events.APIGatewayV2HTTPResponse, error)

// Extract returns a context containing the E2Engine test execution ID from the
// incoming API Gateway HTTP API v2 request.
//
// The execution ID is read from the incoming HTTP headers and, when valid, is
// added to the context using e2engine.WithTestExecutionID. This allows the
// execution ID to be carried through the application and propagated to outgoing
// dependency calls.
//
// If the header is missing, invalid, comma-joined, or appears under multiple
// case-insensitive names, Extract returns the original context unchanged.
//
//nolint:gocritic // Preserve the value-based event API used by Lambda handlers.
func Extract(
	ctx context.Context,
	request events.APIGatewayV2HTTPRequest,
) context.Context {
	id := TestExecutionID(request.Headers)
	if !e2engine.ValidTestExecutionID(id) {
		return ctx
	}

	return e2engine.WithTestExecutionID(ctx, id)
}

// Handler returns an API Gateway HTTP API v2 handler that extracts the E2Engine
// test execution ID from incoming requests and adds it to their contexts.
//
// Handler uses Extract before passing the context and request to the wrapped
// handler. This allows the execution ID to be carried through the application
// using context.Context and later propagated to outgoing dependency calls.
//
// Handler can wrap an existing Lambda handler:
//
//	lambda.Start(Handler(handler.Handle))
//
// If the incoming request does not contain a valid E2Engine test execution ID,
// its context is left unchanged.
func Handler(next APIGatewayV2Handler) APIGatewayV2Handler {
	return func(
		ctx context.Context,
		request events.APIGatewayV2HTTPRequest,
	) (events.APIGatewayV2HTTPResponse, error) {
		return next(
			Extract(ctx, request),
			request,
		)
	}
}

// TestExecutionID returns the E2Engine test execution ID from API Gateway
// HTTP headers.
//
// HTTP header names are case-insensitive. API Gateway HTTP API v2 normally
// provides lowercase header names.
//
// If the header is missing or appears under multiple case-insensitive names,
// TestExecutionID returns an empty string.
//
// TestExecutionID does not validate the returned value.
func TestExecutionID(headers map[string]string) string {
	var id string
	found := false
	for name, value := range headers {
		if strings.EqualFold(name, testExecutionIDHeader) {
			if found {
				return ""
			}
			id = value
			found = true
		}
	}

	return id
}
