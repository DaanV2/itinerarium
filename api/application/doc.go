// Package application holds the service layer: business logic and every
// permission rule (game-day gating, GM-only stripping, existence hiding).
// Services depend on repositories, never on *gorm.DB or HTTP types.
//
// One file (or subpackage, once it grows) per domain area, e.g. users.go,
// characters.go, knowledge.go.
package application
