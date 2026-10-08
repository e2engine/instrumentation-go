package grpc

import (
	"context"

	e2engine "github.com/e2engine/instrumentation-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const testExecutionIDKey = "e2engine-test-execution-id"

// Extract returns a context containing the E2Engine test execution ID from the
// incoming gRPC metadata.
//
// The execution ID is read from the incoming gRPC metadata and, when present,
// is added to the context using e2engine.WithTestExecutionID. This allows the
// execution ID to be carried through the application and propagated to outgoing
// dependency calls.
//
// If the incoming context does not contain an E2Engine test execution ID,
// Extract returns the original context unchanged.
func Extract(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	values := md.Get(testExecutionIDKey)
	if len(values) == 0 || values[0] == "" {
		return ctx
	}

	return e2engine.WithTestExecutionID(
		ctx,
		values[0],
	)
}

// UnaryServerInterceptor returns a gRPC unary server interceptor that extracts
// the E2Engine test execution ID from incoming metadata and adds it to the
// request context.
//
// The interceptor uses Extract before passing the context to the unary handler.
// This allows the execution ID to be carried through the application using
// context.Context and later propagated to outgoing dependency calls using
// Inject, UnaryClientInterceptor, or another E2Engine integration.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		return handler(
			Extract(ctx),
			req,
		)
	}
}

// Inject returns an outgoing context containing the E2Engine test execution ID
// from the context as gRPC metadata.
//
// The execution ID is read from the context and, when present, is added to the
// outgoing gRPC metadata. This allows E2Engine to correlate the outgoing gRPC
// dependency call with the test execution that caused it.
//
// If the context does not contain an E2Engine test execution ID, Inject returns
// the original context unchanged.
func Inject(ctx context.Context) context.Context {
	id := e2engine.TestExecutionID(ctx)
	if id == "" {
		return ctx
	}

	return metadata.AppendToOutgoingContext(
		ctx,
		testExecutionIDKey,
		id,
	)
}

// UnaryClientInterceptor returns a gRPC unary client interceptor that injects
// the E2Engine test execution ID into outgoing gRPC metadata.
//
// The interceptor uses Inject before invoking the RPC. The execution ID must
// therefore be present in the context, typically because the context originated
// from an inbound request processed by UnaryServerInterceptor or Extract.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req any,
		reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		return invoker(
			Inject(ctx),
			method,
			req,
			reply,
			cc,
			opts...,
		)
	}
}

// TestExecutionID returns the E2Engine test execution ID from the gRPC
// metadata.
//
// If the metadata does not contain an E2Engine test execution ID,
// TestExecutionID returns an empty string.
func TestExecutionID(md metadata.MD) string {
	values := md.Get(testExecutionIDKey)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

// WithTestExecutionID returns a copy of the gRPC metadata containing the
// provided E2Engine test execution ID.
//
// If the metadata already contains an E2Engine test execution ID, it is
// replaced. If id is empty, any existing execution ID is removed. The supplied
// metadata is not modified.
func WithTestExecutionID(md metadata.MD, id string) metadata.MD {
	result := md.Copy()

	if id == "" {
		result.Delete(testExecutionIDKey)
		return result
	}

	result.Set(
		testExecutionIDKey,
		id,
	)

	return result
}

// WithoutTestExecutionID returns a copy of the gRPC metadata without the
// E2Engine test execution ID.
//
// The supplied metadata is not modified.
func WithoutTestExecutionID(md metadata.MD) metadata.MD {
	result := md.Copy()
	result.Delete(testExecutionIDKey)

	return result
}
