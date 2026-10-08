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
	if id == "" {
		return next.HandleBuild(ctx, in)
	}

	request, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, metadata, fmt.Errorf(
			"unsupported transport request %T",
			in.Request,
		)
	}

	request.Header = e2enginehttp.WithTestExecutionID(
		request.Header,
		id,
	)

	return next.HandleBuild(ctx, in)
}

// Option returns an AWS SDK configuration option that propagates the E2Engine
// test execution ID to outgoing AWS requests.
//
// The execution ID is read from the context passed to an AWS SDK operation and,
// when present, is added to the outgoing HTTP request. This allows E2Engine to
// correlate AWS dependency calls with the test execution that caused them.
//
// The header is added during the Smithy Build step, before AWS request signing.
// As a result, when an execution ID is present, the E2Engine header is included
// in the AWS SigV4 signed headers. When the context does not contain an E2Engine
// test execution ID, the request is left unchanged.
//
// Option can be supplied directly to config.LoadDefaultConfig:
//
//	cfg, err := config.LoadDefaultConfig(context.Background(), Option())
//
// The context passed to individual AWS SDK operations must carry the execution
// ID for propagation to occur. Typically, that context originates from inbound
// E2Engine instrumentation and is passed through the application to the AWS SDK.
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
