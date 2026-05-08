package testbed_test

import (
	"context"
	"math"
	"math/rand"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/graphql"
	"github.com/hovanhoa/llmgateway/pkg/core/graphql/testbed/client"
	"github.com/hovanhoa/llmgateway/pkg/core/graphql/testbed/server"
	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/hovanhoa/llmgateway/pkg/core/http/testhttp"
)

func BenchmarkEmpty_Direct(b *testing.B) {
	for n := 0; n < b.N; n++ {
		_ = server.GetNotes()
	}
}

func BenchmarkEmpty_LocalService(b *testing.B) {
	// Set up the server
	service := graphql.NewService(server.NewExecutableSchema(
		server.Config{
			Resolvers: &server.Resolver{},
		},
	))

	// Get a client to the server
	ctx := context.Background()
	serviceClient := service.LocalClient()

	// Start the benchmark
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, _ = client.GetNotes(ctx, serviceClient)
	}
}

func BenchmarkEmpty_RemoteService(b *testing.B) {
	// Set up the server
	service := graphql.NewService(server.NewExecutableSchema(
		server.Config{
			Resolvers: &server.Resolver{},
		},
	))

	// Create a test http service
	httpService := http.NewService(
		http.WithLogDropper(func(c *http.Context) bool {
			return true
		}),
	)
	service.MountHTTP(httpService.Router().Group("/prefix"))
	server := testhttp.NewTestHTTPServer(b, httpService)

	// Get a client to the server
	ctx := context.Background()
	serviceClient := graphql.NewRemoteClient(&server, "/prefix/query")

	// Start the benchmark
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, _ = client.GetNotes(ctx, serviceClient)
	}
}

func BenchmarkSize2_Direct(b *testing.B) { runDirectBenchmark(b, 2) }
func BenchmarkSize3_Direct(b *testing.B) { runDirectBenchmark(b, 3) }
func BenchmarkSize4_Direct(b *testing.B) { runDirectBenchmark(b, 4) }
func BenchmarkSize5_Direct(b *testing.B) { runDirectBenchmark(b, 5) }
func BenchmarkSize6_Direct(b *testing.B) { runDirectBenchmark(b, 6) }

func BenchmarkSize2_LocalService(b *testing.B) { runLocalServiceBenchmark(b, 2) }
func BenchmarkSize3_LocalService(b *testing.B) { runLocalServiceBenchmark(b, 3) }
func BenchmarkSize4_LocalService(b *testing.B) { runLocalServiceBenchmark(b, 4) }
func BenchmarkSize5_LocalService(b *testing.B) { runLocalServiceBenchmark(b, 5) }
func BenchmarkSize6_LocalService(b *testing.B) { runLocalServiceBenchmark(b, 6) }

func BenchmarkSize2_RemoteService(b *testing.B) { runRemoteServiceBenchmark(b, 2) }
func BenchmarkSize3_RemoteService(b *testing.B) { runRemoteServiceBenchmark(b, 3) }
func BenchmarkSize4_RemoteService(b *testing.B) { runRemoteServiceBenchmark(b, 4) }
func BenchmarkSize5_RemoteService(b *testing.B) { runRemoteServiceBenchmark(b, 5) }
func BenchmarkSize6_RemoteService(b *testing.B) { runRemoteServiceBenchmark(b, 6) }

func runDirectBenchmark(b *testing.B, i int) {
	// Create the request
	req := client.CreateNoteRequest{
		Title:       "Test",
		Description: randomString(1000 * int(math.Pow(2, float64(i)))),
	}

	// Start the benchmark
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = server.CreateNote(req.Title, req.Description)
	}
}

func runLocalServiceBenchmark(b *testing.B, i int) {
	// Set up the server
	service := graphql.NewService(server.NewExecutableSchema(
		server.Config{
			Resolvers: &server.Resolver{},
		},
	))

	// Get a client to the server
	ctx := context.Background()
	serviceClient := service.LocalClient()

	// Create the request
	req := client.CreateNoteRequest{
		Title:       "Test",
		Description: randomString(50 * int(math.Pow(2, float64(i)))),
	}

	// Start the benchmark
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, _ = client.CreateNote(ctx, serviceClient, req)
	}
}

func runRemoteServiceBenchmark(b *testing.B, i int) {
	// Set up the server
	service := graphql.NewService(server.NewExecutableSchema(
		server.Config{
			Resolvers: &server.Resolver{},
		},
	))

	// Create a test http service
	httpService := http.NewService(
		http.WithLogDropper(func(c *http.Context) bool {
			return true
		}),
	)
	service.MountHTTP(httpService.Router().Group("/prefix"))
	server := testhttp.NewTestHTTPServer(b, httpService)

	// Get a client to the server
	ctx := context.Background()
	serviceClient := graphql.NewRemoteClient(&server, "/prefix/query")

	// Create the request
	req := client.CreateNoteRequest{
		Title:       "Test",
		Description: randomString(50 * int(math.Pow(2, float64(i)))),
	}

	// Start the benchmark
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, _ = client.CreateNote(ctx, serviceClient, req)
	}
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
