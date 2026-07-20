package application

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// requestCacheKey is the private context key the per-request cache is stored
// under.
type requestCacheKey struct{}

// requestCache memoizes the handful of lookups the game-day gating repeats
// many times while serving one request — most importantly the requester's
// character list, which a single document read would otherwise load up to four
// times (roadmap M8). One cache is installed per request by WithRequestCache;
// the gating helpers read it back with requestCacheFrom and fall through to the
// database unmemoized when it is absent (e.g. a direct service call in a test),
// so results are identical either way.
//
// Everything cached here is stable for the lifetime of a single request: the
// requester's characters, the groups those characters belong to, and a group's
// membership are only mutated by dedicated write endpoints that don't also
// serve gated reads, so a request never observes its own stale entry. Cached
// values are shared read-only — callers must not mutate them.
type requestCache struct {
	mu               sync.Mutex
	charactersByUser map[string][]models.Character
	groupIDsByChars  map[string][]string
	groupByID        map[string]*models.Group
}

// WithRequestCache returns a context carrying a fresh per-request gating cache.
// Transport installs one per authenticated request so the gating logic resolves
// the requester's characters and group memberships once instead of re-querying
// them for every access decision (roadmap M8).
func WithRequestCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, requestCacheKey{}, &requestCache{
		charactersByUser: map[string][]models.Character{},
		groupIDsByChars:  map[string][]string{},
		groupByID:        map[string]*models.Group{},
	})
}

// requestCacheFrom returns the per-request cache installed by WithRequestCache,
// or nil when none is present.
func requestCacheFrom(ctx context.Context) *requestCache {
	cache, _ := ctx.Value(requestCacheKey{}).(*requestCache)

	return cache
}

// requesterCharacters loads the requester's characters, memoized per request so
// the gating logic resolves them once (roadmap M8). The returned slice is
// shared and must be treated as read-only.
func requesterCharacters(
	ctx context.Context, characters *repositories.Characters, requester Requester,
) ([]models.Character, error) {
	userID := requester.UserID()

	cache := requestCacheFrom(ctx)
	if cache == nil {
		return characters.ListByUser(ctx, userID)
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cached, ok := cache.charactersByUser[userID]; ok {
		return cached, nil
	}

	loaded, err := characters.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	cache.charactersByUser[userID] = loaded

	return loaded, nil
}

// cachedGroupIDsForCharacters resolves the groups the given characters belong
// to, memoized per request by the character-ID set.
func cachedGroupIDsForCharacters(
	ctx context.Context, groups *repositories.Groups, characterIDs []string,
) ([]string, error) {
	cache := requestCacheFrom(ctx)
	if cache == nil {
		return groups.GroupIDsForCharacters(ctx, characterIDs)
	}

	key := groupIDsKey(characterIDs)

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cached, ok := cache.groupIDsByChars[key]; ok {
		return cached, nil
	}

	loaded, err := groups.GroupIDsForCharacters(ctx, characterIDs)
	if err != nil {
		return nil, err
	}

	cache.groupIDsByChars[key] = loaded

	return loaded, nil
}

// cachedGroup loads a group with its members, memoized per request by ID so a
// group reached through several access decisions is fetched once. The returned
// group is shared and must be treated as read-only.
func cachedGroup(ctx context.Context, groups *repositories.Groups, id string) (*models.Group, error) {
	cache := requestCacheFrom(ctx)
	if cache == nil {
		return groups.GetByID(ctx, id)
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cached, ok := cache.groupByID[id]; ok {
		return cached, nil
	}

	loaded, err := groups.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	cache.groupByID[id] = loaded

	return loaded, nil
}

// groupIDsKey builds an order-independent cache key from a character-ID set.
func groupIDsKey(characterIDs []string) string {
	sorted := make([]string, len(characterIDs))
	copy(sorted, characterIDs)
	sort.Strings(sorted)

	return strings.Join(sorted, ",")
}
