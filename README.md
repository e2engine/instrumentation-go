# E2Engine Go Instrumentation

Go instrumentation for propagating E2Engine test execution IDs through applications and dependency calls.

Part of [E2Engine](https://e2engine.dev), an open-source platform for declarative end-to-end testing.

E2Engine uses a test execution ID to correlate calls made by a system under test with the test execution that caused them. This repository provides lightweight instrumentation for carrying that ID through Go applications and propagating it across supported protocols.

Instrumentation is inactive when no E2Engine test execution ID is present, so the same application binary can be used both with and without E2Engine.

## Requirements

Go 1.25 or later.

## Packages

The repository is split into independent Go modules so applications only need the dependencies required by the protocols they use.

| Module | Purpose |
| --- | --- |
| `github.com/e2engine/instrumentation-go` | Context propagation primitives |
| `github.com/e2engine/instrumentation-go/http` | HTTP server and client instrumentation |
| `github.com/e2engine/instrumentation-go/grpc` | gRPC unary server and client instrumentation |
| `github.com/e2engine/instrumentation-go/aws` | AWS SDK for Go v2 instrumentation |

## How it works

E2Engine attaches a test execution ID to requests sent to the system under test.

Instrumentation extracts that ID at the application boundary, stores it in `context.Context`, and injects it into outgoing dependency calls.

```text
E2Engine
    │
    │ test execution ID
    ▼
┌──────────────────────────┐
│ System under test        │
│                          │
│ inbound instrumentation  │
│            │             │
│            ▼             │
│      context.Context     │
│            │             │
│            ▼             │
│ outbound instrumentation │
└────────────┬─────────────┘
             │
             │ test execution ID
             ▼
        Dependency
```

Application code only needs to propagate `context.Context` normally. The protocol-specific instrumentation handles extraction and injection of the execution ID.

## HTTP

Install the HTTP module:

```bash
go get github.com/e2engine/instrumentation-go/http
```

Instrument incoming requests with `Handler` and outgoing requests with `Transport`:

```go
package main

import (
	"log"
	"net/http"

	e2enginehttp "github.com/e2engine/instrumentation-go/http"
)

func main() {
	// Instrument outgoing HTTP requests to propagate the E2Engine execution ID
	// from the request context to HTTP headers.
	client := &http.Client{
		Transport: e2enginehttp.Transport(),
	}

	// Use the instrumented request context for outgoing dependency calls.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		request, err := http.NewRequestWithContext(
			r.Context(),
			http.MethodGet,
			"http://dependency:8080",
			nil,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response, err := client.Do(request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer response.Body.Close()

		w.WriteHeader(response.StatusCode)
	})

	// Instrument incoming HTTP requests to extract the E2Engine execution ID
	// from HTTP headers into the request context.
	log.Fatal(
		http.ListenAndServe(
			":8080",
			e2enginehttp.Handler(),
		),
	)
}
```

`Handler()` uses `http.DefaultServeMux` by default. An explicit handler can also be supplied:

```go
handler := e2enginehttp.Handler(mux)
```

An existing transport can be wrapped:

```go
client := &http.Client{
	Transport: e2enginehttp.Transport(customTransport),
}
```

The HTTP package also exposes lower-level helpers when direct header manipulation is required:

```go
id := e2enginehttp.TestExecutionID(header)

header = e2enginehttp.WithTestExecutionID(
	header,
	"execution-123",
)

header = e2enginehttp.WithoutTestExecutionID(header)
```

The mutation helpers return copies and do not modify the supplied headers.

## gRPC

Install the gRPC module:

```bash
go get github.com/e2engine/instrumentation-go/grpc
```

E2Engine currently provides instrumentation for unary gRPC calls.

Instrument the server and client with the corresponding interceptors:

```go
package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	e2enginegrpc "github.com/e2engine/instrumentation-go/grpc"
)

func main() {
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
```

The gRPC package also exposes lower-level metadata helpers:

```go
id := e2enginegrpc.TestExecutionID(md)

md = e2enginegrpc.WithTestExecutionID(
	md,
	"execution-123",
)

md = e2enginegrpc.WithoutTestExecutionID(md)
```

The mutation helpers return copies and do not modify the supplied metadata.

## AWS SDK for Go v2

Install the AWS module:

```bash
go get github.com/e2engine/instrumentation-go/aws
```

Add the E2Engine option when loading the AWS configuration:

```go
package main

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"

	e2engineaws "github.com/e2engine/instrumentation-go/aws"
)

func main() {
	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		e2engineaws.Option(),
	)
	if err != nil {
		log.Fatal(err)
	}

	_ = cfg
}
```

The context passed to individual AWS SDK operations must carry the execution ID:

```go
_, err := client.PutItem(
	ctx,
	&dynamodb.PutItemInput{
		// ...
	},
)
```

The instrumentation adds the execution ID during the Smithy Build step, before AWS request signing. When an execution ID is present, the propagation header is therefore included in the AWS SigV4 signed headers.

No request modification is performed when the operation context does not contain an E2Engine test execution ID.

## Context propagation

The root module provides the protocol-independent context API:

```bash
go get github.com/e2engine/instrumentation-go
```

An execution ID can be stored and retrieved directly:

```go
ctx = e2engine.WithTestExecutionID(
	ctx,
	"execution-123",
)

id := e2engine.TestExecutionID(ctx)
```

Most applications should not need to use these functions directly. Protocol instrumentation extracts the execution ID into the context and propagates it to outgoing calls automatically.

The important requirement is to preserve the request context through the application.

For example:

```go
request, err := http.NewRequestWithContext(
	r.Context(),
	http.MethodGet,
	url,
	nil,
)
```

and:

```go
response, err := grpcClient.Call(
	r.Context(),
	request,
)
```

If an application replaces the incoming context with `context.Background()` before making a dependency call, the execution ID is lost and that call cannot be correlated with its E2Engine test execution.

## Using multiple protocols

Instrumentation can be combined freely.

For example, an HTTP service calling both HTTP and gRPC dependencies can use:

```text
HTTP request
     │
     ▼
e2enginehttp.Handler
     │
     ▼
context.Context
     │
     ├───────────────► e2enginehttp.Transport ──► HTTP dependency
     │
     └───────────────► UnaryClientInterceptor ──► gRPC dependency
```

The execution ID remains protocol-neutral while it is carried through the application context. Each outgoing adapter translates it into the representation required by its protocol.

## Production use

E2Engine instrumentation is designed to be safe to include in the normal application binary.

When a request does not contain an E2Engine test execution ID:

- inbound instrumentation leaves the application context unchanged;
- outbound instrumentation does not add propagation information;
- normal application behavior is preserved.

This means a separate E2Engine-specific application build is not required.

## Development

Run tests for all modules:

```bash
make test
```

Run the full verification suite:

```bash
make verify
```

Run tests with coverage:

```bash
make test-cov
```

Run the race detector:

```bash
make test-race
```

The repository tests all public modules against the minimum supported Go version as part of CI.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).
