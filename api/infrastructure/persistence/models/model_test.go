package models_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/google/uuid"
)

func TestBeforeCreateAssignsUUID(t *testing.T) {
	m := &models.Model{}
	if err := m.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate returned error: %v", err)
	}

	if _, err := uuid.Parse(m.ID); err != nil {
		t.Fatalf("expected valid UUID, got %q: %v", m.ID, err)
	}
}

func TestBeforeCreateKeepsExplicitID(t *testing.T) {
	id := uuid.NewString()

	m := &models.Model{ID: id}
	if err := m.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate returned error: %v", err)
	}

	if m.ID != id {
		t.Fatalf("expected ID %q to be kept, got %q", id, m.ID)
	}
}
