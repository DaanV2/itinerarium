package components

import (
	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
)

// Services bundles every application-layer service. Services own the business
// logic and permission rules; the transport layer calls into them.
type Services struct {
	Setup        *application.SetupService
	Auth         *application.AuthService
	Users        *application.UserService
	Characters   *application.CharacterService
	Catalog      *application.CatalogService
	Inventory    *application.InventoryService
	Groups       *application.GroupService
	Locations    *application.LocationService
	Sessions     *application.SessionService
	Repositories *application.RepositoryService
	Documents    *application.DocumentService
	Journals     *application.JournalEntryService
	Activity     *application.ActivityService
}

// NewServices wires the application services over the repositories and token
// service.
func NewServices(repos *Repositories, tokens *authentication.TokenService) *Services {
	characters := application.NewCharacterService(repos.Characters, repos.Users, repos.KnowledgeRepositories)
	locations := application.NewLocationService(
		repos.Locations,
		repos.LocationAccesses,
		repos.Groups,
		repos.Characters,
		characters,
	)

	repositoryService := application.NewRepositoryService(
		repos.KnowledgeRepositories, repos.Groups, repos.Characters,
	)
	documentService := application.NewDocumentService(
		repos.Documents, repositoryService, repos.Characters, repos.Groups, repos.DocumentShares,
	)

	return &Services{
		Setup:      application.NewSetupService(repos.Users, tokens),
		Auth:       application.NewAuthService(tokens, repos.Users),
		Users:      application.NewUserService(repos.Users),
		Characters: characters,
		Catalog:    application.NewCatalogService(repos.Currencies, repos.ItemDefinitions),
		Inventory: application.NewInventoryService(
			characters,
			locations,
			repos.Groups,
			repos.Characters,
			repos.InventoryItems,
			repos.MoneyBalances,
			repos.Currencies,
			repos.ItemDefinitions,
		),
		Groups:       application.NewGroupService(repos.Groups, characters, repos.KnowledgeRepositories),
		Locations:    locations,
		Sessions:     application.NewSessionService(repos.Sessions, characters),
		Repositories: repositoryService,
		Documents:    documentService,
		Journals: application.NewJournalEntryService(
			repos.JournalEntries, repos.Characters, documentService, repos.KnowledgeRepositories,
		),
		Activity: application.NewActivityService(
			repos.ActivityEntries,
			characters,
			repos.Groups,
			repos.LocationAccesses,
			repos.KnowledgeRepositories,
		),
	}
}
