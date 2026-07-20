package application

import (
	"context"
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// ErrInvalidAnnouncement is returned when an announcement is malformed: an
// unknown action, no entity name, a negative game day, or no target at all.
var ErrInvalidAnnouncement = serviceErr(KindValidation, "invalid announcement")

// AnnounceInput carries a GM announcement: an event pushed to specific
// characters, groups, or everyone, surfacing at GameDay regardless of entity
// access. The entity fields are free-form — the thing announced may no longer
// exist (theft, destruction) or may never have been an entity at all.
type AnnounceInput struct {
	GameDay      int
	Action       models.ActivityAction
	EntityType   string
	EntityName   string
	Actor        string
	Public       bool
	CharacterIDs []string
	GroupIDs     []string
}

// ActivityService serves the per-character activity feed, the GM-wide log,
// and GM announcements. It owns the feed's permission rules (core domain
// rules 1–4 applied to activity): game-day gating, entity-access gating via
// each entry's scope, the announced bypass, and server-side actor stripping
// on announced entries for non-GMs.
type ActivityService struct {
	activity   *repositories.ActivityEntries
	characters *CharacterService
	groups     *repositories.Groups
	accesses   *repositories.LocationAccesses
	knowledge  *repositories.KnowledgeRepositories
}

// NewActivityService builds an ActivityService.
func NewActivityService(
	activity *repositories.ActivityEntries,
	characters *CharacterService,
	groups *repositories.Groups,
	accesses *repositories.LocationAccesses,
	knowledge *repositories.KnowledgeRepositories,
) *ActivityService {
	return &ActivityService{
		activity:   activity,
		characters: characters,
		groups:     groups,
		accesses:   accesses,
		knowledge:  knowledge,
	}
}

// Feed returns the activity entries visible to one character, newest first.
// The requester must own the character or be a GM — anyone else gets
// ErrNotFound. An entry surfaces when the character's current_game_day has
// reached it AND either its scope (group, location, repository) is accessible
// to the character, or it is announced to them (directly, via a group, or
// publicly). For non-GM requesters the actor is stripped from announced
// entries before the feed leaves this method (core domain rules 2 and 4).
func (s *ActivityService) Feed(
	ctx context.Context, requester Requester, characterID string,
) ([]models.ActivityEntry, error) {
	character, err := s.characters.Get(ctx, requester, characterID)
	if err != nil {
		return nil, err
	}

	filter, err := s.feedFilter(ctx, character)
	if err != nil {
		return nil, err
	}

	entries, err := s.activity.ListFeed(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("listing activity feed: %w", err)
	}

	if !requester.IsGM() {
		for i := range entries {
			if entries[i].Announced {
				entries[i].Actor = ""
			}
		}
	}

	return entries, nil
}

// feedFilter resolves the scopes a character can reach — its groups, its
// accessible locations, and its visible repositories — into the feed query's
// filter.
func (s *ActivityService) feedFilter(
	ctx context.Context, character *models.Character,
) (*repositories.FeedFilter, error) {
	characterIDs := []string{character.ID}

	groupIDs, err := s.groups.GroupIDsForCharacters(ctx, characterIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving character groups: %w", err)
	}

	locationIDs, err := s.accesses.AccessibleLocationIDs(ctx, characterIDs, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving accessible locations: %w", err)
	}

	repos, err := s.knowledge.ListVisible(ctx, characterIDs, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving visible repositories: %w", err)
	}

	repositoryIDs := make([]string, len(repos))
	for i := range repos {
		repositoryIDs[i] = repos[i].ID
	}

	return &repositories.FeedFilter{
		MaxGameDay:     character.CurrentGameDay,
		GroupIDs:       groupIDs,
		LocationIDs:    locationIDs,
		RepositoryIDs:  repositoryIDs,
		CharacterID:    character.ID,
		TargetGroupIDs: groupIDs,
	}, nil
}

// ListAll returns every activity entry, newest first, with announcement
// targets included. GM only — GMs see all activity regardless of game day.
func (s *ActivityService) ListAll(ctx context.Context, requester Requester) ([]models.ActivityEntry, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}

	entries, err := s.activity.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing activity: %w", err)
	}

	return entries, nil
}

// Announce records a GM broadcast: an announced entry that surfaces to its
// targets at the chosen game day, bypassing entity access but never carrying
// entity content (core domain rule 4). GM only. Every named target must
// exist; the entry carries no scope — its reach comes from the targets alone.
func (s *ActivityService) Announce(
	ctx context.Context, requester Requester, input *AnnounceInput,
) (*models.ActivityEntry, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}
	if err := validateAnnouncement(input); err != nil {
		return nil, err
	}

	targets, err := s.resolveTargets(ctx, requester, input)
	if err != nil {
		return nil, err
	}

	entry := &models.ActivityEntry{
		GameDay:         input.GameDay,
		Action:          input.Action,
		EntityType:      input.EntityType,
		EntityName:      input.EntityName,
		Actor:           input.Actor,
		Announced:       true,
		AnnouncedPublic: input.Public,
		Targets:         targets,
	}
	if err := s.activity.Create(ctx, entry); err != nil {
		return nil, fmt.Errorf("creating announcement: %w", err)
	}

	return entry, nil
}

// validateAnnouncement rejects malformed announcements.
func validateAnnouncement(input *AnnounceInput) error {
	if !input.Action.Valid() {
		return fmt.Errorf("%w: unknown action %q", ErrInvalidAnnouncement, input.Action)
	}
	if input.EntityName == "" {
		return fmt.Errorf("%w: entity name is required", ErrInvalidAnnouncement)
	}
	if input.GameDay < 0 {
		return fmt.Errorf("%w: game day cannot be negative", ErrInvalidAnnouncement)
	}
	if !input.Public && len(input.CharacterIDs) == 0 && len(input.GroupIDs) == 0 {
		return fmt.Errorf("%w: announce publicly or name at least one character or group", ErrInvalidAnnouncement)
	}

	return nil
}

// resolveTargets confirms every named character and group exists and turns
// them into target rows.
func (s *ActivityService) resolveTargets(
	ctx context.Context, requester Requester, input *AnnounceInput,
) ([]models.ActivityTarget, error) {
	targets := make([]models.ActivityTarget, 0, len(input.CharacterIDs)+len(input.GroupIDs))

	for _, id := range input.CharacterIDs {
		if _, err := s.characters.Get(ctx, requester, id); err != nil {
			return nil, err
		}

		characterID := id
		targets = append(targets, models.ActivityTarget{CharacterID: &characterID})
	}
	for _, id := range input.GroupIDs {
		if _, err := s.groups.GetByID(ctx, id); err != nil {
			return nil, notFoundOr(err, "loading group")
		}

		groupID := id
		targets = append(targets, models.ActivityTarget{GroupID: &groupID})
	}

	return targets, nil
}
