package models_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBeforeCreateAssignsUUID(t *testing.T) {
	m := &models.Model{}
	require.NoError(t, m.BeforeCreate(nil))

	_, err := uuid.Parse(m.ID)
	require.NoError(t, err, "expected a valid UUID, got %q", m.ID)
}

func TestBeforeCreateKeepsExplicitID(t *testing.T) {
	id := uuid.NewString()

	m := &models.Model{ID: id}
	require.NoError(t, m.BeforeCreate(nil))

	require.Equal(t, id, m.ID, "explicit ID should be kept")
}
