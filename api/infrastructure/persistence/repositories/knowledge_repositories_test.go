package repositories_test

import (
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

func TestKnowledgeRepositories_EnsureGeneralAndTemplate_AreSingletonsAndIdempotent(t *testing.T) {
	repo := repositories.NewKnowledgeRepositories(newTestDB(t))
	ctx := t.Context()

	general1, err := repo.EnsureGeneral(ctx)
	if err != nil {
		t.Fatalf("EnsureGeneral: %v", err)
	}
	general2, err := repo.EnsureGeneral(ctx)
	if err != nil {
		t.Fatalf("EnsureGeneral (second): %v", err)
	}
	if general1.ID != general2.ID {
		t.Fatalf("EnsureGeneral created a second row: %s != %s", general1.ID, general2.ID)
	}

	template, err := repo.EnsureTemplate(ctx)
	if err != nil {
		t.Fatalf("EnsureTemplate: %v", err)
	}
	if template.ID == general1.ID {
		t.Fatal("template repository reused the general repository's ID")
	}

	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("List returned %d repositories, want 2", len(all))
	}
}

func TestKnowledgeRepositories_EnsureForGroup_OnePerGroup(t *testing.T) {
	repo := repositories.NewKnowledgeRepositories(newTestDB(t))
	ctx := t.Context()

	first, err := repo.EnsureForGroup(ctx, "group-1")
	if err != nil {
		t.Fatalf("EnsureForGroup: %v", err)
	}
	if first.Type != models.RepositoryTypeGroup || first.GroupID == nil || *first.GroupID != "group-1" {
		t.Fatalf("unexpected repository: %+v", first)
	}

	second, err := repo.EnsureForGroup(ctx, "group-1")
	if err != nil {
		t.Fatalf("EnsureForGroup (second): %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("EnsureForGroup created a second row for the same group: %s != %s", first.ID, second.ID)
	}

	other, err := repo.EnsureForGroup(ctx, "group-2")
	if err != nil {
		t.Fatalf("EnsureForGroup(group-2): %v", err)
	}
	if other.ID == first.ID {
		t.Fatal("two different groups shared a repository")
	}
}

func TestKnowledgeRepositories_EnsureForCharacter_OnePerCharacter(t *testing.T) {
	repo := repositories.NewKnowledgeRepositories(newTestDB(t))
	ctx := t.Context()

	first, err := repo.EnsureForCharacter(ctx, "char-1")
	if err != nil {
		t.Fatalf("EnsureForCharacter: %v", err)
	}
	if first.Type != models.RepositoryTypeCharacter || first.CharacterID == nil || *first.CharacterID != "char-1" {
		t.Fatalf("unexpected repository: %+v", first)
	}

	second, err := repo.EnsureForCharacter(ctx, "char-1")
	if err != nil {
		t.Fatalf("EnsureForCharacter (second): %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("EnsureForCharacter created a second row for the same character: %s != %s", first.ID, second.ID)
	}
}

func TestKnowledgeRepositories_GetByID_NotFound(t *testing.T) {
	repo := repositories.NewKnowledgeRepositories(newTestDB(t))

	if _, err := repo.GetByID(t.Context(), "does-not-exist"); !errors.Is(err, repositories.ErrNotFound) {
		t.Fatalf("GetByID(missing) = %v, want ErrNotFound", err)
	}
}

func TestKnowledgeRepositories_ListVisible(t *testing.T) {
	repo := repositories.NewKnowledgeRepositories(newTestDB(t))
	ctx := t.Context()

	if _, err := repo.EnsureGeneral(ctx); err != nil {
		t.Fatalf("EnsureGeneral: %v", err)
	}
	if _, err := repo.EnsureTemplate(ctx); err != nil {
		t.Fatalf("EnsureTemplate: %v", err)
	}
	if _, err := repo.EnsureForCharacter(ctx, "char-1"); err != nil {
		t.Fatalf("EnsureForCharacter: %v", err)
	}
	if _, err := repo.EnsureForCharacter(ctx, "char-2"); err != nil {
		t.Fatalf("EnsureForCharacter(char-2): %v", err)
	}
	if _, err := repo.EnsureForGroup(ctx, "group-1"); err != nil {
		t.Fatalf("EnsureForGroup: %v", err)
	}

	visible, err := repo.ListVisible(ctx, []string{"char-1"}, []string{"group-1"})
	if err != nil {
		t.Fatalf("ListVisible: %v", err)
	}

	// general + template + char-1's own repository + group-1's repository —
	// char-2's repository must not appear.
	if len(visible) != 4 {
		t.Fatalf("ListVisible returned %d repositories, want 4: %+v", len(visible), visible)
	}
	for _, r := range visible {
		if r.CharacterID != nil && *r.CharacterID == "char-2" {
			t.Fatal("ListVisible leaked another character's repository")
		}
	}
}
