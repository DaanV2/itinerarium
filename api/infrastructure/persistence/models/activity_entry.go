package models

// ActivityAction is what happened to the entity an ActivityEntry describes.
type ActivityAction string

// Actions recorded so far. M5 adds added/updated/removed/destroyed/stolen for
// inventories and documents.
const (
	ActivityActionJoined ActivityAction = "joined"
	ActivityActionLeft   ActivityAction = "left"
)

// ActivityEntry is one append-only event in the campaign log, stamped with the
// game day at which it happened. M2 records group membership changes; M5 turns
// the log into the per-character activity feed (with game-day gating, entity
// access checks, and announcements) and extends the tracked events.
//
// Actor is the display name of who caused the event. Per core domain rule 2 it
// must be stripped server-side for non-GM readers on announced entries — that
// stripping lands with the M5 feed, since no read endpoint exists yet.
type ActivityEntry struct {
	Model
	GameDay     int            `gorm:"not null" json:"game_day"`
	Action      ActivityAction `gorm:"not null" json:"action"`
	EntityType  string         `gorm:"not null;index:idx_activity_entity" json:"entity_type"`
	EntityID    string         `gorm:"type:uuid;not null;index:idx_activity_entity" json:"entity_id"`
	EntityName  string         `gorm:"not null" json:"entity_name"`
	Actor       string         `json:"actor,omitempty"`
	CharacterID string         `gorm:"type:uuid;index" json:"character_id,omitempty"`
}
