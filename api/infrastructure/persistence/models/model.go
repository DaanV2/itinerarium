// Package models holds the GORM models. Every entity embeds [Model] and must
// be registered in persistence/migrations.go to get a table.
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Model is the shared base for all persisted entities: UUID primary key
// (generated in BeforeCreate), timestamps, and soft delete.
type Model struct {
	ID        string `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate assigns a UUID unless the caller set one explicitly.
func (m *Model) BeforeCreate(_ *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}

	return nil
}
