package grpc_test

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	e2enginegrpc "github.com/e2engine/instrumentation-go/grpc"
)

func ExampleUnaryServerInterceptor() {
	// Instrument outgoing unary calls to propagate the E2Engine execution ID
	// from the call context to gRPC metadata.
	connection, err := grpc.NewClient(
		"dependency:8081",
		grpc.WithTransportCredentials(
			insecure.NewCredentials(),
		),
		grpc.WithUnaryInterceptor(
			e2enginegrpc.UnaryClientInterceptor(),
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer connection.Close()

	// The service implementation can use connection normally as long as it
	// propagates the incoming request context to dependency calls.

	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal(err)
	}

	// Instrument incoming unary calls to extract the E2Engine execution ID
	// from gRPC metadata into the request context.
	server := grpc.NewServer(
		grpc.UnaryInterceptor(
			e2enginegrpc.UnaryServerInterceptor(),
		),
	)

	// Register the service with server.

	log.Fatal(server.Serve(listener))
}
