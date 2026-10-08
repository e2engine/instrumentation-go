package aws_test

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	e2engineaws "github.com/e2engine/instrumentation-go/aws"
)

func ExampleOption() {
	// import e2engineaws "github.com/e2engine/instrumentation-go/aws"

	cfg, err := config.LoadDefaultConfig(context.TODO(), e2engineaws.Option())
	if err != nil {
		log.Fatal(err)
	}
	_ = cfg
}
