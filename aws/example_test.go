package aws_test

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"

	e2engineaws "github.com/e2engine/instrumentation-go/aws"
)

func ExampleOption() {
	// One valid explicit outgoing execution ID overrides the operation context.
	// Otherwise, a valid context ID is used, or invalid/multiple IDs are removed.
	// Pass the incoming request context to individual SDK operations.

	cfg, err := config.LoadDefaultConfig(context.TODO(), e2engineaws.Option())
	if err != nil {
		log.Fatal(err)
	}
	_ = cfg
}
