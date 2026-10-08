package e2engine

import (
	"context"
	"testing"
)

func TestWithTestExecutionID(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		wantID string
	}{
		{
			name:   "execution ID",
			id:     "execution-123",
			wantID: "execution-123",
		},
		{
			name: "empty execution ID",
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

			if tt.id == "" && got != ctx {
				t.Fatal(
					"WithTestExecutionID() returned a new context for an empty execution ID",
				)
			}
		})
	}
}

func TestWithTestExecutionID_OverridesExecutionID(t *testing.T) {
	ctx := WithTestExecutionID(
		context.Background(),
		"execution-123",
	)

	ctx = WithTestExecutionID(
		ctx,
		"execution-456",
	)

	if id := TestExecutionID(ctx); id != "execution-456" {
		t.Fatalf(
			"TestExecutionID() = %q, want %q",
			id,
			"execution-456",
		)
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
