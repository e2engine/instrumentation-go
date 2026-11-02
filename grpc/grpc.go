package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	e2engine "github.com/e2engine/instrumentation-go"
)

const testExecutionIDKey = "e2engine-test-execution-id"

// Extract returns a context containing the E2Engine test execution ID from the
// incoming gRPC metadata.
//
// The execution ID is read from the incoming gRPC metadata and, when valid,
// is added to the context using e2engine.WithTestExecutionID. This allows the
// execution ID to be carried through the application and propagated to outgoing
// dependency calls.
//
// If the incoming metadata is missing an execution ID, or contains an invalid
// ID or multiple values, Extract returns the original context unchanged.
func Extract(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	values := md.Get(testExecutionIDKey)
	if len(values) != 1 ||
		!e2engine.ValidTestExecutionID(values[0]) {
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

// Inject returns a context containing a valid E2Engine test execution ID
// as outgoing gRPC metadata.
//
// Exactly one valid outgoing metadata value takes precedence over the execution
// ID stored in the context. If the outgoing ID is absent, invalid, or contains
// multiple values, a valid context ID is used instead. If neither source
// provides a valid ID, any invalid or multiple execution ID values are removed.
//
// Inject never modifies caller-owned metadata. If no changes are necessary,
// the original context is returned unchanged.
func Inject(ctx context.Context) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)
	values, exists := md[testExecutionIDKey]
	if len(values) == 1 && e2engine.ValidTestExecutionID(values[0]) {
		return ctx
	}

	id := e2engine.TestExecutionID(ctx)
	validContextID := e2engine.ValidTestExecutionID(id)
	if !exists && !validContextID {
		return ctx
	}

	// FromOutgoingContext returns a copy, so these changes cannot modify the
	// original metadata, including values added with AppendToOutgoingContext.
	if validContextID {
		if md == nil {
			md = make(metadata.MD)
		}
		md.Set(testExecutionIDKey, id)
	} else {
		md.Delete(testExecutionIDKey)
	}

	return metadata.NewOutgoingContext(ctx, md)
}

// UnaryClientInterceptor returns a gRPC unary client interceptor that injects
// the E2Engine test execution ID into outgoing gRPC metadata.
//
// The interceptor uses Inject before invoking the RPC. Exactly one valid ID in
// outgoing metadata takes precedence over the context ID. Otherwise, a valid
// context ID is used, typically originating from UnaryServerInterceptor or
// Extract. Without a valid fallback, invalid or multiple outgoing ID values are
// removed. Unrelated metadata and the original context are preserved.
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
// TestExecutionID returns an empty string. The returned value is not validated.
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
// If id is valid, it replaces any existing execution ID. If id is empty or
// invalid, any existing execution ID is removed. The supplied metadata is
// not modified.
func WithTestExecutionID(md metadata.MD, id string) metadata.MD {
	result := md.Copy()

	if !e2engine.ValidTestExecutionID(id) {
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
