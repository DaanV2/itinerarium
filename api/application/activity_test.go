package application_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type activityTestEnv struct {
	activity   *application.ActivityService
	inventory  *application.InventoryService
	docs       *application.DocumentService
	repos      *application.RepositoryService
	groups     *application.GroupService
	locations  *application.LocationService
	characters *application.CharacterService
	currencies *repositories.Currencies
}

// newActivityTestEnv wires the full service stack over one in-memory
// database, mirroring components/services.go — activity tracking cuts across
// inventories, money, documents, and groups.
func newActivityTestEnv(t *testing.T) activityTestEnv {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err, "New persistence")
	require.NoError(t, db.Migrate())

	knowledgeRepo := repositories.NewKnowledgeRepositories(db)
	characterRepo := repositories.NewCharacters(db)
	groupRepo := repositories.NewGroups(db)
	accessRepo := repositories.NewLocationAccesses(db)
	currencies := repositories.NewCurrencies(db)

	charSvc := application.NewCharacterService(characterRepo, repositories.NewUsers(db), knowledgeRepo)
	locationSvc := application.NewLocationService(
		repositories.NewLocations(db), accessRepo, groupRepo, characterRepo, charSvc,
	)
	groupSvc := application.NewGroupService(groupRepo, charSvc, knowledgeRepo)
	repoSvc := application.NewRepositoryService(knowledgeRepo, groupRepo, characterRepo)
	docSvc := application.NewDocumentService(
		repositories.NewDocuments(db), repoSvc, characterRepo, groupRepo, repositories.NewDocumentShares(db),
	)
	inventorySvc := application.NewInventoryService(
		charSvc, locationSvc, groupRepo, characterRepo,
		repositories.NewInventoryItems(db), repositories.NewMoneyBalances(db),
		currencies, repositories.NewItemDefinitions(db),
	)
	activitySvc := application.NewActivityService(
		repositories.NewActivityEntries(db), charSvc, groupRepo, accessRepo, knowledgeRepo,
	)

	err = repoSvc.EnsureSystemRepositories(t.Context())
	require.NoError(t, err, "EnsureSystemRepositories")

	return activityTestEnv{
		activity:   activitySvc,
		inventory:  inventorySvc,
		docs:       docSvc,
		repos:      repoSvc,
		groups:     groupSvc,
		locations:  locationSvc,
		characters: charSvc,
		currencies: currencies,
	}
}

func (e activityTestEnv) newCharacter(t *testing.T, owner application.Requester, name string, day int) *models.Character {
	t.Helper()

	character, err := e.characters.Create(t.Context(), owner, "", name)
	require.NoError(t, err, "set game day")

	if day != 0 {
		_, err := e.characters.Update(t.Context(), gmRequester, character.ID, nil, &day)
		require.NoError(t, err, "set game day")

		character.CurrentGameDay = day
	}

	return character
}

func (e activityTestEnv) setGameDay(t *testing.T, characterID string, day int) {
	t.Helper()

	_, err := e.characters.Update(t.Context(), gmRequester, characterID, nil, &day)
	require.NoError(t, err, "set game day")
}

func (e activityTestEnv) feed(
	t *testing.T, requester application.Requester, characterID string,
) []models.ActivityEntry {
	t.Helper()

	entries, err := e.activity.Feed(t.Context(), requester, characterID)
	require.NoError(t, err, "Feed")

	return entries
}

func groupOwner(id string) models.InventoryOwner { return models.InventoryOwner{GroupID: &id} }
func locOwner(id string) models.InventoryOwner   { return models.InventoryOwner{LocationID: &id} }
func charOwner(id string) models.InventoryOwner  { return models.InventoryOwner{CharacterID: &id} }

func findEntry(entries []models.ActivityEntry, action models.ActivityAction, name string) *models.ActivityEntry {
	for i := range entries {
		if entries[i].Action == action && entries[i].EntityName == name {
			return &entries[i]
		}
	}

	return nil
}

func TestActivityService_Feed_ForeignCharacterHidden(t *testing.T) {
	env := newActivityTestEnv(t)
	character := env.newCharacter(t, playerRequester, "Aria", 0)
	outsider := fakeRequester{id: "outsider-1", gm: false}

	_, err := env.activity.Feed(t.Context(), outsider, character.ID)

	require.ErrorIs(t, err, application.ErrNotFound, "GM can see any character feed")
}

func TestActivityService_Feed_GroupScopeGatedByMembershipAndGameDay(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	member := env.newCharacter(t, playerRequester, "Aria", 5)
	otherRequester := fakeRequester{id: "player-2", gm: false}
	outsider := env.newCharacter(t, otherRequester, "Beren", 10)

	group := createGroup(t, env.groups, "Thieves Guild")
	err := env.groups.Join(ctx, playerRequester, group.ID, member.ID)
	require.NoError(t, err)

	// GM stamps at the group's present day — member's day 5.
	_, err = env.inventory.AddItem(ctx, gmRequester, groupOwner(group.ID), "Lockpicks", nil, 3, "")
	require.NoError(t, err)

	memberFeed := env.feed(t, playerRequester, member.ID)
	assert.NotNil(t, findEntry(memberFeed, models.ActivityActionAdded, "Lockpicks"),
		"member feed = %+v, want the Lockpicks addition", memberFeed)

	joinEntry := findEntry(memberFeed, models.ActivityActionJoined, "Thieves Guild")
	require.NotNil(t, joinEntry, "member feed = %+v, want the join event", memberFeed)
	assert.Equal(t, "Aria", joinEntry.Actor)

	// A non-member sees nothing — group content stays invisible (rule 3).
	outsiderFeed := env.feed(t, otherRequester, outsider.ID)
	assert.Empty(t, outsiderFeed, "outsider feed = %+v, want empty", outsiderFeed)

	// Rewinding below the events' game day makes them disappear again (M4).
	env.setGameDay(t, member.ID, 4)
	rewound := env.feed(t, playerRequester, member.ID)
	assert.Empty(t, rewound, "rewound feed = %+v, want empty", rewound)
}

func TestActivityService_Feed_PlayerChangeStampedWithOwnCharacter(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	actor := env.newCharacter(t, playerRequester, "Aria", 3)
	otherRequester := fakeRequester{id: "player-2", gm: false}
	ahead := env.newCharacter(t, otherRequester, "Beren", 10)

	group := createGroup(t, env.groups, "Caravan")
	for _, c := range []*models.Character{actor, ahead} {
		err := env.groups.Join(ctx, gmRequester, group.ID, c.ID)
		require.NoError(t, err)
	}

	_, err := env.inventory.AddItem(ctx, playerRequester, groupOwner(group.ID), "Rations", nil, 5, "")
	require.NoError(t, err)

	entry := findEntry(env.feed(t, playerRequester, actor.ID), models.ActivityActionAdded, "Rations")
	require.NotNil(t, entry, "actor cannot see their own addition")
	assert.Equal(t, 3, entry.GameDay)
	assert.Equal(t, "Aria", entry.Actor)

	// The member further along sees it too (3 <= 10).
	assert.NotNil(t, findEntry(env.feed(t, otherRequester, ahead.ID), models.ActivityActionAdded, "Rations"),
		"fellow member cannot see the addition")
}

func TestActivityService_Feed_LocationScopeRequiresAccess(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	granted := env.newCharacter(t, playerRequester, "Aria", 5)
	otherRequester := fakeRequester{id: "player-2", gm: false}
	denied := env.newCharacter(t, otherRequester, "Beren", 5)

	location, err := env.locations.Create(ctx, gmRequester, "The Vault", "")
	require.NoError(t, err)
	_, err = env.locations.GrantAccess(ctx, gmRequester, location.Location.ID, &granted.ID, nil)
	require.NoError(t, err)

	_, err = env.inventory.AddItem(ctx, gmRequester, locOwner(location.Location.ID), "Gold Idol", nil, 1, "")
	require.NoError(t, err)

	assert.NotNil(t, findEntry(env.feed(t, playerRequester, granted.ID), models.ActivityActionAdded, "Gold Idol"),
		"granted character cannot see the location event")

	deniedFeed := env.feed(t, otherRequester, denied.ID)
	assert.Empty(t, deniedFeed, "denied feed = %+v, want empty — location existence must not leak", deniedFeed)
}

func TestActivityService_Feed_CharacterInventoryNotTracked(t *testing.T) {
	env := newActivityTestEnv(t)
	character := env.newCharacter(t, playerRequester, "Aria", 5)

	_, err := env.inventory.AddItem(
		t.Context(), playerRequester, charOwner(character.ID), "Diary", nil, 1, "",
	)
	require.NoError(t, err)

	feed := env.feed(t, playerRequester, character.ID)
	assert.Empty(t, feed, "feed = %+v, want empty — personal inventories are private", feed)
}

func TestActivityService_Feed_DocumentEventsSurfaceAtRevealDay(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	character := env.newCharacter(t, playerRequester, "Aria", 3)
	general, err := env.repos.List(ctx, gmRequester)
	require.NoError(t, err)

	var generalID string
	for i := range general {
		if general[i].Type == models.RepositoryTypeGeneral {
			generalID = general[i].ID
		}
	}

	day := 5
	_, err = env.docs.Create(ctx, gmRequester, generalID, &application.CreateDocumentInput{
		Path:            "lore/prophecy",
		Title:           "The Prophecy",
		SharedOnGameDay: &day,
		Sections:        []application.DocumentSectionInput{{Content: "It is foretold."}},
	})
	require.NoError(t, err)

	// Before the reveal day the entry must not leak the document's existence.
	feed := env.feed(t, playerRequester, character.ID)
	assert.Empty(t, feed, "feed before reveal = %+v, want empty", feed)

	env.setGameDay(t, character.ID, 5)
	assert.NotNil(t, findEntry(env.feed(t, playerRequester, character.ID), models.ActivityActionAdded, "The Prophecy"),
		"document addition missing from feed after reveal day")
}

func TestActivityService_Feed_GroupMoneyChangeTracked(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	member := env.newCharacter(t, playerRequester, "Aria", 5)
	group := createGroup(t, env.groups, "Merchant House")
	err := env.groups.Join(ctx, playerRequester, group.ID, member.ID)
	require.NoError(t, err)

	currency := &models.Currency{Code: "gp", Name: "Gold", Ratio: 100}
	err = env.currencies.Create(ctx, currency)
	require.NoError(t, err)

	_, err = env.inventory.SetMoney(ctx, gmRequester, groupOwner(group.ID), currency.ID, 250)
	require.NoError(t, err)

	entry := findEntry(env.feed(t, playerRequester, member.ID), models.ActivityActionUpdated, "Gold")
	require.NotNil(t, entry, "money change missing from member feed")
	assert.Equal(t, "money", entry.EntityType)
	assert.Equal(t, "GM", entry.Actor)
}

func TestActivityService_Announce_BypassesAccessAndStripsActor(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	target := env.newCharacter(t, playerRequester, "Aria", 5)
	otherRequester := fakeRequester{id: "player-2", gm: false}
	bystander := env.newCharacter(t, otherRequester, "Beren", 5)

	_, err := env.activity.Announce(ctx, gmRequester, &application.AnnounceInput{
		GameDay:      4,
		Action:       models.ActivityActionStolen,
		EntityType:   "item",
		EntityName:   "The Ruby of Vess",
		Actor:        "The Grey Hand",
		CharacterIDs: []string{target.ID},
	})
	require.NoError(t, err)

	// The target sees the theft — without the actor (rules 2 and 4).
	entry := findEntry(env.feed(t, playerRequester, target.ID), models.ActivityActionStolen, "The Ruby of Vess")
	require.NotNil(t, entry, "announced theft missing from target feed")
	assert.Empty(t, entry.Actor, "actor leaked to player")

	// A GM reading the same feed keeps the full picture.
	gmEntry := findEntry(env.feed(t, gmRequester, target.ID), models.ActivityActionStolen, "The Ruby of Vess")
	require.NotNil(t, gmEntry, "GM view of announced entry missing")
	assert.Equal(t, "The Grey Hand", gmEntry.Actor)

	// Characters outside the target list never see it.
	bystanderFeed := env.feed(t, otherRequester, bystander.ID)
	assert.Empty(t, bystanderFeed, "bystander feed = %+v, want empty", bystanderFeed)
}

func TestActivityService_Announce_PublicReachesEveryoneAtGameDay(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	behind := env.newCharacter(t, playerRequester, "Aria", 3)
	otherRequester := fakeRequester{id: "player-2", gm: false}
	ahead := env.newCharacter(t, otherRequester, "Beren", 10)

	_, err := env.activity.Announce(ctx, gmRequester, &application.AnnounceInput{
		GameDay:    5,
		Action:     models.ActivityActionDestroyed,
		EntityName: "The Old Bridge",
		Public:     true,
	})
	require.NoError(t, err)

	// Public announcements still respect the surfacing game day.
	feed := env.feed(t, playerRequester, behind.ID)
	assert.Empty(t, feed, "feed before the announcement day = %+v, want empty", feed)
	assert.NotNil(t, findEntry(env.feed(t, otherRequester, ahead.ID), models.ActivityActionDestroyed, "The Old Bridge"),
		"public announcement missing from feed past its day")
}

func TestActivityService_Announce_GroupTargetFollowsMembership(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	member := env.newCharacter(t, playerRequester, "Aria", 5)
	group := createGroup(t, env.groups, "Order of the Gauntlet")
	err := env.groups.Join(ctx, playerRequester, group.ID, member.ID)
	require.NoError(t, err)

	_, err = env.activity.Announce(ctx, gmRequester, &application.AnnounceInput{
		GameDay:    5,
		Action:     models.ActivityActionStolen,
		EntityName: "The Reliquary",
		GroupIDs:   []string{group.ID},
	})
	require.NoError(t, err)

	assert.NotNil(t, findEntry(env.feed(t, playerRequester, member.ID), models.ActivityActionStolen, "The Reliquary"),
		"group-targeted announcement missing from member feed")
}

func TestActivityService_Announce_Validation(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	_, err := env.activity.Announce(ctx, playerRequester, &application.AnnounceInput{
		GameDay: 1, Action: models.ActivityActionStolen, EntityName: "X", Public: true,
	})
	require.ErrorIs(t, err, application.ErrForbidden)

	_, err = env.activity.Announce(ctx, gmRequester, &application.AnnounceInput{
		GameDay: 1, Action: models.ActivityActionStolen, EntityName: "X",
	})
	require.ErrorIs(t, err, application.ErrInvalidAnnouncement)

	_, err = env.activity.Announce(ctx, gmRequester, &application.AnnounceInput{
		GameDay: 1, Action: models.ActivityAction("evaporated"), EntityName: "X", Public: true,
	})
	require.ErrorIs(t, err, application.ErrInvalidAnnouncement)

	_, err = env.activity.Announce(ctx, gmRequester, &application.AnnounceInput{
		GameDay: 1, Action: models.ActivityActionStolen, EntityName: "X",
		CharacterIDs: []string{"no-such-character"},
	})
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestActivityService_ListAll_GMOnlySeesEverything(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	_, err := env.activity.ListAll(ctx, playerRequester)
	require.ErrorIs(t, err, application.ErrForbidden)

	member := env.newCharacter(t, playerRequester, "Aria", 99)
	group := createGroup(t, env.groups, "Night Watch")
	err = env.groups.Join(ctx, playerRequester, group.ID, member.ID)
	require.NoError(t, err)
	_, err = env.activity.Announce(ctx, gmRequester, &application.AnnounceInput{
		GameDay: 500, Action: models.ActivityActionDestroyed, EntityName: "The Beacon",
		CharacterIDs: []string{member.ID},
	})
	require.NoError(t, err)

	entries, err := env.activity.ListAll(ctx, gmRequester)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "ListAll should return join + announcement")

	announced := findEntry(entries, models.ActivityActionDestroyed, "The Beacon")
	require.NotNil(t, announced, "announcement missing from GM log — GMs see all activity regardless of game day")
	require.Len(t, announced.Targets, 1)
	require.NotNil(t, announced.Targets[0].CharacterID)
	assert.Equal(t, member.ID, *announced.Targets[0].CharacterID, "announcement targets = %+v, want the member", announced.Targets)
}
