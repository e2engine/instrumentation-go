package e2engine

import "context"

type testExecutionIDKey struct{}

// WithTestExecutionID returns a context containing the provided E2Engine test
// execution ID.
//
// If id is empty, the original context is returned unchanged.
func WithTestExecutionID(
	ctx context.Context,
	id string,
) context.Context {
	if id == "" {
		return ctx
	}

	return context.WithValue(
		ctx,
		testExecutionIDKey{},
		id,
	)
}

// TestExecutionID returns the E2Engine test execution ID stored in the context.
//
// If the context does not contain an E2Engine test execution ID,
// TestExecutionID returns an empty string.
func TestExecutionID(ctx context.Context) string {
	value, _ := ctx.Value(testExecutionIDKey{}).(string)
	return value
}
