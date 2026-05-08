package server

import (
	"time"

	"github.com/hovanhoa/llmgateway/pkg/core/encoding"
	"github.com/hovanhoa/llmgateway/pkg/core/graphql/testbed/server/model"
)

var (
	notes = make(map[string]*model.Note)
)

func GetNoteByID(id string) *model.Note {
	return notes[id]
}

func GetNotes() (allNotes []model.Note) {
	for _, v := range notes {
		allNotes = append(allNotes, *v)
	}

	return
}

func CreateNote(title, description string) (note model.Note) {
	id := encoding.NewRandomIdentifier("note")
	note = model.Note{
		ID:          id,
		Title:       title,
		Description: description,
		CreatedAt:   time.Now(),
	}

	notes[id] = &note
	return
}

func DeleteNoteByID(id string) *model.Note {
	note := GetNoteByID(id)
	delete(notes, id)
	return note
}
