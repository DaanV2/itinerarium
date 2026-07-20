package application_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// These tests pin down the query behavior M8 hardened: the gating logic
// resolves the requester's characters once per request (roadmap M8, item 1),
// and ListSharedWithMe batch-loads instead of issuing a query per share (item
// 2). They fail loudly if a future change reintroduces the per-request or
// per-row re-querying, rather than leaving the regression to be re-discovered.

// queryCounter tallies the database read queries GORM issues, per table, so a
// test can assert exactly how many a service call costs.
type queryCounter struct {
	mu       sync.Mutex
	perTable map[string]int
	total    int
}

// attachQueryCounter registers GORM callbacks that count every query and row
// read against db. It counts reads (Query/Row processors); writes have their
// own processors and are not relevant to the N+1 read patterns under test.
func attachQueryCounter(t *testing.T, db *persistence.Database) *queryCounter {
	t.Helper()

	qc := &queryCounter{perTable: map[string]int{}}
	inc := func(g *gorm.DB) {
		qc.mu.Lock()
		defer qc.mu.Unlock()

		qc.total++
		if g.Statement != nil && g.Statement.Table != "" {
			qc.perTable[g.Statement.Table]++
		}
	}

	require.NoError(t, db.DB().Callback().Query().Register("qc:query", inc))
	require.NoError(t, db.DB().Callback().Row().Register("qc:row", inc))

	return qc
}

func (qc *queryCounter) reset() {
	qc.mu.Lock()
	defer qc.mu.Unlock()

	qc.perTable = map[string]int{}
	qc.total = 0
}

func (qc *queryCounter) table(name string) int {
	qc.mu.Lock()
	defer qc.mu.Unlock()

	return qc.perTable[name]
}

func (qc *queryCounter) count() int {
	qc.mu.Lock()
	defer qc.mu.Unlock()

	return qc.total
}

// perfTestEnv wires the document stack against a fresh in-memory database and
// exposes that database so a query counter can be attached to it.
type perfTestEnv struct {
	db         *persistence.Database
	docs       *application.DocumentService
	repos      *application.RepositoryService
	characters *application.CharacterService
}

func newPerfTestEnv(t *testing.T) perfTestEnv {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)
	require.NoError(t, db.Migrate())

	knowledgeRepo := repositories.NewKnowledgeRepositories(db)
	characterRepo := repositories.NewCharacters(db)
	groupRepo := repositories.NewGroups(db)

	charSvc := application.NewCharacterService(characterRepo, repositories.NewUsers(db), knowledgeRepo)
	repoSvc := application.NewRepositoryService(knowledgeRepo, groupRepo, characterRepo)
	docSvc := application.NewDocumentService(
		repositories.NewDocuments(db), repoSvc, characterRepo, groupRepo, repositories.NewDocumentShares(db),
	)

	require.NoError(t, repoSvc.EnsureSystemRepositories(t.Context()))

	return perfTestEnv{db: db, docs: docSvc, repos: repoSvc, characters: charSvc}
}

func (e perfTestEnv) generalRepo(t *testing.T) *models.Repository {
	t.Helper()

	repos, err := e.repos.List(t.Context(), gmRequester)
	require.NoError(t, err)

	for i := range repos {
		if repos[i].Type == models.RepositoryTypeGeneral {
			return &repos[i]
		}
	}

	require.Fail(t, "general repository not found")

	return nil
}

// TestListByUser_ResolvedOncePerDocumentUpdate proves the per-request gating
// cache collapses the several character-list loads a single document update
// used to issue — getAccessible's game-day gate and documentEntry each
// re-queried them — down to one (roadmap M8, item 1). A general repository is
// used so the only reads against the characters table are the requester's own
// character list (a group's member preload also reads that table, which would
// blur the count).
func TestListByUser_ResolvedOncePerDocumentUpdate(t *testing.T) {
	env := newPerfTestEnv(t)
	setupCtx := t.Context()

	_, err := env.characters.Create(setupCtx, playerRequester, "", "Aria")
	require.NoError(t, err)

	general := env.generalRepo(t)
	view, err := env.docs.Create(setupCtx, gmRequester, general.ID, &application.CreateDocumentInput{
		Path:     "lore/history",
		Sections: []application.DocumentSectionInput{{Content: "In the beginning."}},
	})
	require.NoError(t, err)

	qc := attachQueryCounter(t, env.db)

	// A real request installs the cache; measure the update as that request.
	ctx := application.WithRequestCache(setupCtx)
	qc.reset()

	_, err = env.docs.Update(ctx, playerRequester, view.Document.ID, &application.UpdateDocumentInput{
		Path:     "lore/history",
		Sections: []application.DocumentSectionInput{{Content: "In the beginning, twice."}},
	})
	require.NoError(t, err)

	assert.Equal(t, 1, qc.table("characters"),
		"the requester's characters should be loaded exactly once per request")
}

// TestListSharedWithMe_QueryCountConstant proves ListSharedWithMe issues the
// same number of queries no matter how many documents are shared — the N+1
// (a document load and a repository load per share) is gone (roadmap M8, item
// 2).
func TestListSharedWithMe_QueryCountConstant(t *testing.T) {
	few := sharedWithMeQueryCount(t, 2)
	many := sharedWithMeQueryCount(t, 8)

	assert.Equal(t, few, many,
		"ListSharedWithMe query count must not grow with the number of shared documents")
}

// sharedWithMeQueryCount shares shareCount documents with a fresh player's
// character and returns how many read queries ListSharedWithMe issues.
func sharedWithMeQueryCount(t *testing.T, shareCount int) int {
	t.Helper()

	env := newPerfTestEnv(t)
	setupCtx := t.Context()

	character, err := env.characters.Create(setupCtx, playerRequester, "", "Aria")
	require.NoError(t, err)

	general := env.generalRepo(t)
	for i := range shareCount {
		view, err := env.docs.Create(setupCtx, gmRequester, general.ID, &application.CreateDocumentInput{
			Path:     fmt.Sprintf("shared/doc-%d", i),
			Sections: []application.DocumentSectionInput{{Content: "Secret."}},
		})
		require.NoError(t, err)

		_, err = env.docs.ShareWithCharacter(setupCtx, gmRequester, view.Document.ID, character.ID, 0)
		require.NoError(t, err)
	}

	qc := attachQueryCounter(t, env.db)
	ctx := application.WithRequestCache(setupCtx)
	qc.reset()

	views, err := env.docs.ListSharedWithMe(ctx, playerRequester)
	require.NoError(t, err)
	require.Len(t, views, shareCount)

	return qc.count()
}
