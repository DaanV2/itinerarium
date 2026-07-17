package application_test

import (
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
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
	if err != nil {
		t.Fatalf("persistence.New: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

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

	if err := repoSvc.EnsureSystemRepositories(t.Context()); err != nil {
		t.Fatalf("EnsureSystemRepositories: %v", err)
	}

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
	if err != nil {
		t.Fatalf("Create character: %v", err)
	}
	if day != 0 {
		if _, err := e.characters.Update(t.Context(), gmRequester, character.ID, nil, &day); err != nil {
			t.Fatalf("set game day: %v", err)
		}

		character.CurrentGameDay = day
	}

	return character
}

func (e activityTestEnv) setGameDay(t *testing.T, characterID string, day int) {
	t.Helper()

	if _, err := e.characters.Update(t.Context(), gmRequester, characterID, nil, &day); err != nil {
		t.Fatalf("set game day: %v", err)
	}
}

func (e activityTestEnv) feed(
	t *testing.T, requester application.Requester, characterID string,
) []models.ActivityEntry {
	t.Helper()

	entries, err := e.activity.Feed(t.Context(), requester, characterID)
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}

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
	if _, err := env.activity.Feed(t.Context(), outsider, character.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Feed for foreign character = %v, want ErrNotFound", err)
	}
}

func TestActivityService_Feed_GroupScopeGatedByMembershipAndGameDay(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	member := env.newCharacter(t, playerRequester, "Aria", 5)
	otherRequester := fakeRequester{id: "player-2", gm: false}
	outsider := env.newCharacter(t, otherRequester, "Beren", 10)

	group := createGroup(t, env.groups, "Thieves Guild")
	if err := env.groups.Join(ctx, playerRequester, group.ID, member.ID); err != nil {
		t.Fatalf("Join: %v", err)
	}

	// GM stamps at the group's present day — member's day 5.
	if _, err := env.inventory.AddItem(ctx, gmRequester, groupOwner(group.ID), "Lockpicks", nil, 3, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	memberFeed := env.feed(t, playerRequester, member.ID)
	if findEntry(memberFeed, models.ActivityActionAdded, "Lockpicks") == nil {
		t.Fatalf("member feed = %+v, want the Lockpicks addition", memberFeed)
	}
	if entry := findEntry(memberFeed, models.ActivityActionJoined, "Thieves Guild"); entry == nil {
		t.Fatalf("member feed = %+v, want the join event", memberFeed)
	} else if entry.Actor != "Aria" {
		t.Fatalf("join entry actor = %q, want %q", entry.Actor, "Aria")
	}

	// A non-member sees nothing — group content stays invisible (rule 3).
	if outsiderFeed := env.feed(t, otherRequester, outsider.ID); len(outsiderFeed) != 0 {
		t.Fatalf("outsider feed = %+v, want empty", outsiderFeed)
	}

	// Rewinding below the events' game day makes them disappear again (M4).
	env.setGameDay(t, member.ID, 4)
	if rewound := env.feed(t, playerRequester, member.ID); len(rewound) != 0 {
		t.Fatalf("rewound feed = %+v, want empty", rewound)
	}
}

func TestActivityService_Feed_PlayerChangeStampedWithOwnCharacter(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	actor := env.newCharacter(t, playerRequester, "Aria", 3)
	otherRequester := fakeRequester{id: "player-2", gm: false}
	ahead := env.newCharacter(t, otherRequester, "Beren", 10)

	group := createGroup(t, env.groups, "Caravan")
	for _, c := range []*models.Character{actor, ahead} {
		if err := env.groups.Join(ctx, gmRequester, group.ID, c.ID); err != nil {
			t.Fatalf("Join: %v", err)
		}
	}

	if _, err := env.inventory.AddItem(ctx, playerRequester, groupOwner(group.ID), "Rations", nil, 5, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	entry := findEntry(env.feed(t, playerRequester, actor.ID), models.ActivityActionAdded, "Rations")
	if entry == nil {
		t.Fatalf("actor cannot see their own addition")
	}
	if entry.GameDay != 3 || entry.Actor != "Aria" {
		t.Fatalf("entry = day %d actor %q, want day 3 actor Aria", entry.GameDay, entry.Actor)
	}

	// The member further along sees it too (3 <= 10).
	if findEntry(env.feed(t, otherRequester, ahead.ID), models.ActivityActionAdded, "Rations") == nil {
		t.Fatalf("fellow member cannot see the addition")
	}
}

func TestActivityService_Feed_LocationScopeRequiresAccess(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	granted := env.newCharacter(t, playerRequester, "Aria", 5)
	otherRequester := fakeRequester{id: "player-2", gm: false}
	denied := env.newCharacter(t, otherRequester, "Beren", 5)

	location, err := env.locations.Create(ctx, gmRequester, "The Vault", "", "")
	if err != nil {
		t.Fatalf("Create location: %v", err)
	}
	if _, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, &granted.ID, nil); err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}

	if _, err := env.inventory.AddItem(ctx, gmRequester, locOwner(location.ID), "Gold Idol", nil, 1, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	if findEntry(env.feed(t, playerRequester, granted.ID), models.ActivityActionAdded, "Gold Idol") == nil {
		t.Fatalf("granted character cannot see the location event")
	}
	if deniedFeed := env.feed(t, otherRequester, denied.ID); len(deniedFeed) != 0 {
		t.Fatalf("denied feed = %+v, want empty — location existence must not leak", deniedFeed)
	}
}

func TestActivityService_Feed_CharacterInventoryNotTracked(t *testing.T) {
	env := newActivityTestEnv(t)
	character := env.newCharacter(t, playerRequester, "Aria", 5)

	if _, err := env.inventory.AddItem(
		t.Context(), playerRequester, charOwner(character.ID), "Diary", nil, 1, "",
	); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	if feed := env.feed(t, playerRequester, character.ID); len(feed) != 0 {
		t.Fatalf("feed = %+v, want empty — personal inventories are private", feed)
	}
}

func TestActivityService_Feed_DocumentEventsSurfaceAtRevealDay(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	character := env.newCharacter(t, playerRequester, "Aria", 3)
	general, err := env.repos.List(ctx, gmRequester)
	if err != nil {
		t.Fatalf("List repositories: %v", err)
	}

	var generalID string
	for i := range general {
		if general[i].Type == models.RepositoryTypeGeneral {
			generalID = general[i].ID
		}
	}

	day := 5
	if _, err := env.docs.Create(ctx, gmRequester, generalID, &application.CreateDocumentInput{
		Path:            "lore/prophecy",
		Title:           "The Prophecy",
		SharedOnGameDay: &day,
		Sections:        []application.DocumentSectionInput{{Content: "It is foretold."}},
	}); err != nil {
		t.Fatalf("Create document: %v", err)
	}

	// Before the reveal day the entry must not leak the document's existence.
	if feed := env.feed(t, playerRequester, character.ID); len(feed) != 0 {
		t.Fatalf("feed before reveal = %+v, want empty", feed)
	}

	env.setGameDay(t, character.ID, 5)
	if findEntry(env.feed(t, playerRequester, character.ID), models.ActivityActionAdded, "The Prophecy") == nil {
		t.Fatalf("document addition missing from feed after reveal day")
	}
}

func TestActivityService_Feed_GroupMoneyChangeTracked(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	member := env.newCharacter(t, playerRequester, "Aria", 5)
	group := createGroup(t, env.groups, "Merchant House")
	if err := env.groups.Join(ctx, playerRequester, group.ID, member.ID); err != nil {
		t.Fatalf("Join: %v", err)
	}

	currency := &models.Currency{Code: "gp", Name: "Gold", Ratio: 100}
	if err := env.currencies.Create(ctx, currency); err != nil {
		t.Fatalf("Create currency: %v", err)
	}

	if _, err := env.inventory.SetMoney(ctx, gmRequester, groupOwner(group.ID), currency.ID, 250); err != nil {
		t.Fatalf("SetMoney: %v", err)
	}

	entry := findEntry(env.feed(t, playerRequester, member.ID), models.ActivityActionUpdated, "Gold")
	if entry == nil {
		t.Fatalf("money change missing from member feed")
	}
	if entry.EntityType != "money" || entry.Actor != "GM" {
		t.Fatalf("entry = type %q actor %q, want money/GM", entry.EntityType, entry.Actor)
	}
}

func TestActivityService_Announce_BypassesAccessAndStripsActor(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	target := env.newCharacter(t, playerRequester, "Aria", 5)
	otherRequester := fakeRequester{id: "player-2", gm: false}
	bystander := env.newCharacter(t, otherRequester, "Beren", 5)

	if _, err := env.activity.Announce(ctx, gmRequester, &application.AnnounceInput{
		GameDay:      4,
		Action:       models.ActivityActionStolen,
		EntityType:   "item",
		EntityName:   "The Ruby of Vess",
		Actor:        "The Grey Hand",
		CharacterIDs: []string{target.ID},
	}); err != nil {
		t.Fatalf("Announce: %v", err)
	}

	// The target sees the theft — without the actor (rules 2 and 4).
	entry := findEntry(env.feed(t, playerRequester, target.ID), models.ActivityActionStolen, "The Ruby of Vess")
	if entry == nil {
		t.Fatalf("announced theft missing from target feed")
	}
	if entry.Actor != "" {
		t.Fatalf("actor leaked to player: %q", entry.Actor)
	}

	// A GM reading the same feed keeps the full picture.
	gmEntry := findEntry(env.feed(t, gmRequester, target.ID), models.ActivityActionStolen, "The Ruby of Vess")
	if gmEntry == nil || gmEntry.Actor != "The Grey Hand" {
		t.Fatalf("GM view of announced entry = %+v, want actor preserved", gmEntry)
	}

	// Characters outside the target list never see it.
	if bystanderFeed := env.feed(t, otherRequester, bystander.ID); len(bystanderFeed) != 0 {
		t.Fatalf("bystander feed = %+v, want empty", bystanderFeed)
	}
}

func TestActivityService_Announce_PublicReachesEveryoneAtGameDay(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	behind := env.newCharacter(t, playerRequester, "Aria", 3)
	otherRequester := fakeRequester{id: "player-2", gm: false}
	ahead := env.newCharacter(t, otherRequester, "Beren", 10)

	if _, err := env.activity.Announce(ctx, gmRequester, &application.AnnounceInput{
		GameDay:    5,
		Action:     models.ActivityActionDestroyed,
		EntityName: "The Old Bridge",
		Public:     true,
	}); err != nil {
		t.Fatalf("Announce: %v", err)
	}

	// Public announcements still respect the surfacing game day.
	if feed := env.feed(t, playerRequester, behind.ID); len(feed) != 0 {
		t.Fatalf("feed before the announcement day = %+v, want empty", feed)
	}
	if findEntry(env.feed(t, otherRequester, ahead.ID), models.ActivityActionDestroyed, "The Old Bridge") == nil {
		t.Fatalf("public announcement missing from feed past its day")
	}
}

func TestActivityService_Announce_GroupTargetFollowsMembership(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	member := env.newCharacter(t, playerRequester, "Aria", 5)
	group := createGroup(t, env.groups, "Order of the Gauntlet")
	if err := env.groups.Join(ctx, playerRequester, group.ID, member.ID); err != nil {
		t.Fatalf("Join: %v", err)
	}

	if _, err := env.activity.Announce(ctx, gmRequester, &application.AnnounceInput{
		GameDay:    5,
		Action:     models.ActivityActionStolen,
		EntityName: "The Reliquary",
		GroupIDs:   []string{group.ID},
	}); err != nil {
		t.Fatalf("Announce: %v", err)
	}

	if findEntry(env.feed(t, playerRequester, member.ID), models.ActivityActionStolen, "The Reliquary") == nil {
		t.Fatalf("group-targeted announcement missing from member feed")
	}
}

func TestActivityService_Announce_Validation(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	if _, err := env.activity.Announce(ctx, playerRequester, &application.AnnounceInput{
		GameDay: 1, Action: models.ActivityActionStolen, EntityName: "X", Public: true,
	}); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("Announce as player = %v, want ErrForbidden", err)
	}

	if _, err := env.activity.Announce(ctx, gmRequester, &application.AnnounceInput{
		GameDay: 1, Action: models.ActivityActionStolen, EntityName: "X",
	}); !errors.Is(err, application.ErrInvalidAnnouncement) {
		t.Fatalf("Announce without targets = %v, want ErrInvalidAnnouncement", err)
	}

	if _, err := env.activity.Announce(ctx, gmRequester, &application.AnnounceInput{
		GameDay: 1, Action: models.ActivityAction("evaporated"), EntityName: "X", Public: true,
	}); !errors.Is(err, application.ErrInvalidAnnouncement) {
		t.Fatalf("Announce with unknown action = %v, want ErrInvalidAnnouncement", err)
	}

	if _, err := env.activity.Announce(ctx, gmRequester, &application.AnnounceInput{
		GameDay: 1, Action: models.ActivityActionStolen, EntityName: "X",
		CharacterIDs: []string{"no-such-character"},
	}); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Announce to unknown character = %v, want ErrNotFound", err)
	}
}

func TestActivityService_ListAll_GMOnlySeesEverything(t *testing.T) {
	env := newActivityTestEnv(t)
	ctx := t.Context()

	if _, err := env.activity.ListAll(ctx, playerRequester); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("ListAll as player = %v, want ErrForbidden", err)
	}

	member := env.newCharacter(t, playerRequester, "Aria", 99)
	group := createGroup(t, env.groups, "Night Watch")
	if err := env.groups.Join(ctx, playerRequester, group.ID, member.ID); err != nil {
		t.Fatalf("Join: %v", err)
	}
	if _, err := env.activity.Announce(ctx, gmRequester, &application.AnnounceInput{
		GameDay: 500, Action: models.ActivityActionDestroyed, EntityName: "The Beacon",
		CharacterIDs: []string{member.ID},
	}); err != nil {
		t.Fatalf("Announce: %v", err)
	}

	entries, err := env.activity.ListAll(ctx, gmRequester)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ListAll returned %d entries, want 2 (join + announcement)", len(entries))
	}

	announced := findEntry(entries, models.ActivityActionDestroyed, "The Beacon")
	if announced == nil {
		t.Fatalf("announcement missing from GM log — GMs see all activity regardless of game day")
	}
	if len(announced.Targets) != 1 || announced.Targets[0].CharacterID == nil ||
		*announced.Targets[0].CharacterID != member.ID {
		t.Fatalf("announcement targets = %+v, want the member", announced.Targets)
	}
}
