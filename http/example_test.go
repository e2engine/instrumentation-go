package http_test

import (
	"log"
	"net/http"

	e2enginehttp "github.com/e2engine/instrumentation-go/http"
)

func ExampleHandler() {
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
	log.Fatal(http.ListenAndServe(":8080", e2enginehttp.Handler()))
}
