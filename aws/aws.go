package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	e2engine "github.com/e2engine/instrumentation-go"
	e2enginehttp "github.com/e2engine/instrumentation-go/http"
)

type middleware struct{}

func (middleware) ID() string {
	return "E2EnginePropagation"
}

func (middleware) HandleBuild(
	ctx context.Context,
	in smithymiddleware.BuildInput,
	next smithymiddleware.BuildHandler,
) (
	out smithymiddleware.BuildOutput,
	metadata smithymiddleware.Metadata,
	err error,
) {
	id := e2engine.TestExecutionID(ctx)
	request, ok := in.Request.(*smithyhttp.Request)
	if !ok || request == nil || request.Request == nil {
		if !e2engine.ValidTestExecutionID(id) {
			return next.HandleBuild(ctx, in)
		}

		return out, metadata, fmt.Errorf(
			"unsupported transport request %T",
			in.Request,
		)
	}

	// The operation context carries the fallback ID; the Smithy request's own
	// context may differ. Inject preserves valid explicit headers and copies
	// the HTTP request only when a change is needed.
	source := request.WithContext(ctx)
	injected := e2enginehttp.Inject(source)
	if injected != source {
		// A shallow Smithy copy preserves its stream and transport fields. The
		// HTTP request and headers were already copied by Inject.
		result := *request
		result.Request = injected.WithContext(request.Context())
		in.Request = &result
	}

	return next.HandleBuild(ctx, in)
}

// Option returns an AWS SDK configuration option that propagates the E2Engine
// test execution ID to outgoing AWS requests.
//
// Exactly one valid execution ID in the outgoing HTTP headers takes precedence
// over the ID in the operation context. If the header is missing, invalid, or
// contains multiple values, a valid context ID is used instead. Without a valid
// fallback, invalid or multiple execution ID values are removed.
//
// The header is added during the Smithy Build step, before AWS request signing.
// As a result, when an execution ID is present, the E2Engine header is included
// in the AWS SigV4 signed headers. The original request is preserved; when no
// changes are necessary, the same request is passed to the next middleware.
//
// Option can be supplied directly to config.LoadDefaultConfig:
//
//	cfg, err := config.LoadDefaultConfig(context.Background(), Option())
//
// The context passed to individual AWS SDK operations supplies the fallback ID.
// Typically, that context originates from inbound E2Engine instrumentation and
// is passed through the application to the AWS SDK.
func Option() func(*config.LoadOptions) error {
	return config.WithAPIOptions(
		[]func(*smithymiddleware.Stack) error{
			func(stack *smithymiddleware.Stack) error {
				return stack.Build.Add(
					middleware{},
					smithymiddleware.After,
				)
			},
		},
	)
}
