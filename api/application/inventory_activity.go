package application

import (
	"context"
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
)

// activityActorGM is the actor name recorded when a GM (who has no character
// of their own) makes a change.
const activityActorGM = "GM"

// Changes to shared inventories and money (groups and locations) are recorded
// in the activity log (roadmap M5); personal character inventories are
// private and produce no entries. Built entries are handed to the repository
// call that performs the change, so event and change commit in one
// transaction (the M2 membership precedent).
//
// Game-day stamping: a player's change is stamped with their acting
// character's current_game_day (the qualifying character furthest along), so
// they always see their own events. A GM's change is stamped with the scope's
// "present day" — the highest current_game_day among characters with access —
// so it surfaces to everyone there while characters still catching up see it
// once their own day arrives.

// inventoryEntry builds the activity-log row for one change to a group or
// location inventory (or a group money balance) as a zero-or-one-element
// slice ready to hand to a repository call. Character owners yield no entry —
// nothing to record.
func (s *InventoryService) inventoryEntry(
	ctx context.Context, requester Requester, owner models.InventoryOwner,
	action models.ActivityAction, entityType, entityID, entityName string,
) ([]*models.ActivityEntry, error) {
	var (
		scopeType, scopeID, actor string
		day                       int
		err                       error
	)

	switch {
	case owner.GroupID != nil:
		scopeType, scopeID = models.ActivityScopeGroup, *owner.GroupID
		actor, day, err = s.groupActor(ctx, requester, *owner.GroupID)
	case owner.LocationID != nil:
		scopeType, scopeID = models.ActivityScopeLocation, *owner.LocationID
		actor, day, err = s.locationActor(ctx, requester, *owner.LocationID)
	default:
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return []*models.ActivityEntry{{
		GameDay:    day,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		EntityName: entityName,
		Actor:      actor,
		ScopeType:  scopeType,
		ScopeID:    scopeID,
	}}, nil
}

// groupActor resolves who a group-scoped event is attributed to and the game
// day it is stamped with (see the stamping rule above).
func (s *InventoryService) groupActor(
	ctx context.Context, requester Requester, groupID string,
) (actor string, day int, err error) {
	group, err := s.groups.GetByID(ctx, groupID)
	if err != nil {
		return "", 0, fmt.Errorf("loading group: %w", err)
	}

	frontier := 0
	memberIDs := make(map[string]struct{}, len(group.Members))
	for i := range group.Members {
		memberIDs[group.Members[i].ID] = struct{}{}
		if group.Members[i].CurrentGameDay > frontier {
			frontier = group.Members[i].CurrentGameDay
		}
	}

	if requester.IsGM() {
		return activityActorGM, frontier, nil
	}

	characters, err := s.characterRepo.ListByUser(ctx, requester.UserID())
	if err != nil {
		return "", 0, fmt.Errorf("listing requester characters: %w", err)
	}

	var best *models.Character
	for i := range characters {
		if _, member := memberIDs[characters[i].ID]; !member {
			continue
		}
		if best == nil || characters[i].CurrentGameDay > best.CurrentGameDay {
			best = &characters[i]
		}
	}
	if best == nil {
		// Access was verified before any entry is built, so this only happens
		// in exotic races; record an anonymous event at the group's present.
		return "", frontier, nil
	}

	return best.Name, best.CurrentGameDay, nil
}

// locationActor resolves who a location-scoped event is attributed to and the
// game day it is stamped with (see the stamping rule above).
func (s *InventoryService) locationActor(
	ctx context.Context, requester Requester, locationID string,
) (actor string, day int, err error) {
	if requester.IsGM() {
		frontier, err := s.locations.FrontierGameDay(ctx, locationID)
		if err != nil {
			return "", 0, err
		}

		return activityActorGM, frontier, nil
	}

	characters, err := s.characterRepo.ListByUser(ctx, requester.UserID())
	if err != nil {
		return "", 0, fmt.Errorf("listing requester characters: %w", err)
	}

	var best *models.Character
	for i := range characters {
		ok, err := s.locations.AccessibleToCharacter(ctx, characters[i].ID, locationID)
		if err != nil {
			return "", 0, err
		}
		if !ok {
			continue
		}
		if best == nil || characters[i].CurrentGameDay > best.CurrentGameDay {
			best = &characters[i]
		}
	}
	if best == nil {
		frontier, err := s.locations.FrontierGameDay(ctx, locationID)
		if err != nil {
			return "", 0, err
		}

		return "", frontier, nil
	}

	return best.Name, best.CurrentGameDay, nil
}
