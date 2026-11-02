package e2engine

import (
	"context"
	"strings"
	"testing"
)

func TestWithTestExecutionID(t *testing.T) {
	validID := strings.Repeat("a1b2c3d4", 8)

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
			name: "empty execution ID",
		},
		{
			name: "execution ID too short",
			id:   strings.Repeat("a", 63),
		},
		{
			name: "execution ID too long",
			id:   strings.Repeat("a", 65),
		},
		{
			name: "uppercase execution ID",
			id:   strings.Repeat("a", 63) + "A",
		},
		{
			name: "execution ID with invalid characters",
			id:   strings.Repeat("a", 63) + "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			got := WithTestExecutionID(ctx, tt.id)

			if id := TestExecutionID(got); id != tt.wantID {
				t.Fatalf(
					"TestExecutionID() = %q, want %q",
					id,
					tt.wantID,
				)
			}

			if !ValidTestExecutionID(tt.id) && got != ctx {
				t.Fatal(
					"WithTestExecutionID() returned a new context for an invalid execution ID",
				)
			}

			if ValidTestExecutionID(tt.id) && got == ctx {
				t.Fatal(
					"WithTestExecutionID() returned the original context for a valid execution ID",
				)
			}
		})
	}
}

func TestWithTestExecutionID_OverridesExecutionID(t *testing.T) {
	firstID := strings.Repeat("a", 64)
	secondID := strings.Repeat("b", 64)

	ctx := WithTestExecutionID(
		context.Background(),
		firstID,
	)

	ctx = WithTestExecutionID(
		ctx,
		secondID,
	)

	if id := TestExecutionID(ctx); id != secondID {
		t.Fatalf(
			"TestExecutionID() = %q, want %q",
			id,
			secondID,
		)
	}
}

func TestWithTestExecutionID_PreservesExecutionID(t *testing.T) {
	validID := strings.Repeat("a", 64)

	tests := []struct {
		name string
		id   string
	}{
		{
			name: "empty execution ID",
		},
		{
			name: "invalid execution ID",
			id:   "execution-123",
		},
		{
			name: "invalid uppercase execution ID",
			id:   strings.Repeat("A", 64),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithTestExecutionID(
				context.Background(),
				validID,
			)

			got := WithTestExecutionID(ctx, tt.id)

			if got != ctx {
				t.Fatal(
					"WithTestExecutionID() replaced the context for an invalid execution ID",
				)
			}

			if id := TestExecutionID(got); id != validID {
				t.Fatalf(
					"TestExecutionID() = %q, want %q",
					id,
					validID,
				)
			}
		})
	}
}

func TestTestExecutionID(t *testing.T) {
	if id := TestExecutionID(context.Background()); id != "" {
		t.Fatalf(
			"TestExecutionID() = %q, want empty",
			id,
		)
	}
}
