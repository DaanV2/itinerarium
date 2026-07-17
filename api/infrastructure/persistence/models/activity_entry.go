package models

// ActivityAction is what happened to the entity an ActivityEntry describes.
type ActivityAction string

// The known activity actions. joined/left track group membership (M2);
// added/updated/removed track inventories, money, and documents (M5);
// destroyed and stolen are used on GM announcements (M5).
const (
	ActivityActionJoined    ActivityAction = "joined"
	ActivityActionLeft      ActivityAction = "left"
	ActivityActionAdded     ActivityAction = "added"
	ActivityActionUpdated   ActivityAction = "updated"
	ActivityActionRemoved   ActivityAction = "removed"
	ActivityActionDestroyed ActivityAction = "destroyed"
	ActivityActionStolen    ActivityAction = "stolen"
)

// Valid reports whether a is one of the known activity actions.
func (a ActivityAction) Valid() bool {
	switch a {
	case ActivityActionJoined, ActivityActionLeft, ActivityActionAdded,
		ActivityActionUpdated, ActivityActionRemoved, ActivityActionDestroyed, ActivityActionStolen:
		return true
	default:
		return false
	}
}

// Activity scope types: the kind of entity whose access rule gates who may
// normally see an entry (core domain rule 1 applied to the feed). Announced
// entries carry no scope — their reach comes from their targets instead.
const (
	ActivityScopeGroup      = "group"
	ActivityScopeLocation   = "location"
	ActivityScopeRepository = "repository"
)

// ActivityEntry is one append-only event in the campaign log, stamped with the
// game day at which it happened. A character sees an entry through one of two
// paths (enforced in the application layer, see architecture.md):
//
//  1. Normal: current_game_day >= GameDay AND the character has access to the
//     entry's scope entity (their groups, their locations, repositories they
//     can reach).
//  2. Announced: the entry is announced to them (directly, via a group, or
//     publicly) and their game day has been reached — entity access is
//     bypassed, and the Actor field is stripped server-side for non-GMs
//     (core domain rules 2 and 4).
//
// Actor is the display name of who caused the event (a character name, or
// "GM"). EntityType/EntityID/EntityName describe what changed (an item, a
// document, a money balance, or the group joined/left).
type ActivityEntry struct {
	Model
	GameDay     int            `gorm:"not null" json:"game_day"`
	Action      ActivityAction `gorm:"not null" json:"action"`
	EntityType  string         `gorm:"not null;index:idx_activity_entity" json:"entity_type"`
	EntityID    string         `gorm:"type:uuid;not null;index:idx_activity_entity" json:"entity_id"`
	EntityName  string         `gorm:"not null" json:"entity_name"`
	Actor       string         `json:"actor,omitempty"`
	CharacterID string         `gorm:"type:uuid;index" json:"character_id,omitempty"`

	// Access scope for the normal visibility path. Empty on announced-only
	// entries (GM broadcasts about things that may no longer exist).
	ScopeType string `gorm:"index:idx_activity_scope" json:"scope_type,omitempty"`
	ScopeID   string `gorm:"type:uuid;index:idx_activity_scope" json:"scope_id,omitempty"`

	// Announcement fields. An announced entry surfaces to its targets at
	// GameDay regardless of entity access (never revealing entity content).
	Announced       bool             `gorm:"not null;default:false" json:"announced"`
	AnnouncedPublic bool             `gorm:"not null;default:false" json:"announced_public,omitempty"`
	Targets         []ActivityTarget `json:"-"`
}

// ActivityTarget is one explicit recipient of an announced ActivityEntry:
// exactly one character or one group.
type ActivityTarget struct {
	Model
	ActivityEntryID string  `gorm:"type:uuid;index;not null" json:"activity_entry_id"`
	CharacterID     *string `gorm:"type:uuid;index" json:"character_id,omitempty"`
	GroupID         *string `gorm:"type:uuid;index" json:"group_id,omitempty"`
}
