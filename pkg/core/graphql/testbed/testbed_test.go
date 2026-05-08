// Package testbed implements a test bed that sets up a GraphQL service,
// a GraphQL client, implements some test resolvers, and demonstrates
// the two parts working together both locally and over HTTP.
package testbed_test

import (
	"context"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/graphql"
	"github.com/hovanhoa/llmgateway/pkg/core/graphql/testbed/client"
	"github.com/hovanhoa/llmgateway/pkg/core/graphql/testbed/server"
	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/hovanhoa/llmgateway/pkg/core/http/testhttp"
	"github.com/stretchr/testify/assert"
)

func TestLocal(t *testing.T) {
	// Set up the server
	service := graphql.NewService(server.NewExecutableSchema(
		server.Config{
			Resolvers: &server.Resolver{},
		},
	))

	// Get a client to the server
	serviceClient := service.LocalClient()

	// Make some requests to the service
	runTest(t, serviceClient)
}

func TestRemote(t *testing.T) {
	// Set up the server
	service := graphql.NewService(server.NewExecutableSchema(
		server.Config{
			Resolvers: &server.Resolver{},
		},
	))

	// Create a test http service
	httpService := http.NewService()
	service.MountHTTP(httpService.Router().Group("/prefix"))
	server := testhttp.NewTestHTTPServer(t, httpService)

	// Get a client to the server
	serviceClient := graphql.NewRemoteClient(&server, "/prefix/query")

	// Make some requests to the service
	runTest(t, serviceClient)
}

func runTest(t *testing.T, serviceClient graphql.Client) {
	ctx := context.Background()

	// Get all the notes - there should be none
	gotNotes, err := client.GetNotes(ctx, serviceClient)
	assert.NoError(t, err)
	assert.Empty(t, gotNotes.Notes)

	// Create a note
	note, err := client.CreateNote(ctx, serviceClient, client.CreateNoteRequest{
		Title:       "Test",
		Description: "This is a note",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, note.CreateNote.Id)
	assert.Equal(t, "Test", note.CreateNote.Title)
	assert.Equal(t, "This is a note", note.CreateNote.Description)

	// Fetch the created note
	gotNote, err := client.GetNoteById(ctx, serviceClient, note.CreateNote.Id)
	assert.NoError(t, err)
	assert.Equal(t, note.CreateNote.Id, gotNote.Note.Id)

	// Fetch all notes - there should be one
	gotNotes, err = client.GetNotes(ctx, serviceClient)
	assert.NoError(t, err)
	assert.Len(t, gotNotes.Notes, 1)
	assert.Equal(t, note.CreateNote.Id, gotNotes.Notes[0].Id)

	// Try deleting a note that does not exist - should return an error
	_, err = client.DeleteNoteById(ctx, serviceClient, "nonexistent_id")
	assert.ErrorContains(t, err, "note not found")

	// Try deleting a note that exists
	deletedNote, err := client.DeleteNoteById(ctx, serviceClient, note.CreateNote.Id)
	assert.NoError(t, err)
	assert.Equal(t, note.CreateNote.Id, deletedNote.DeleteNote.Id)

	// Fetch all notes - there should be none
	gotNotes, err = client.GetNotes(ctx, serviceClient)
	assert.NoError(t, err)
	assert.Empty(t, gotNotes.Notes)
}
