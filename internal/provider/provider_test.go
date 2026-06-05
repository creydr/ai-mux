package provider

import "testing"

func TestParseRepoRef_Valid(t *testing.T) {
	ref, err := ParseRepoRef("owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Owner != "owner" {
		t.Errorf("expected owner, got %s", ref.Owner)
	}
	if ref.Repo != "repo" {
		t.Errorf("expected repo, got %s", ref.Repo)
	}
}

func TestParseRepoRef_WithOrg(t *testing.T) {
	ref, err := ParseRepoRef("my-org/my-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Owner != "my-org" {
		t.Errorf("expected my-org, got %s", ref.Owner)
	}
	if ref.Repo != "my-repo" {
		t.Errorf("expected my-repo, got %s", ref.Repo)
	}
}

func TestParseRepoRef_MissingSlash(t *testing.T) {
	_, err := ParseRepoRef("noslash")
	if err == nil {
		t.Fatal("expected error for missing slash")
	}
}

func TestParseRepoRef_Empty(t *testing.T) {
	_, err := ParseRepoRef("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
}

func TestParseRepoRef_EmptyOwner(t *testing.T) {
	_, err := ParseRepoRef("/repo")
	if err == nil {
		t.Fatal("expected error for empty owner")
	}
}

func TestParseRepoRef_EmptyRepo(t *testing.T) {
	_, err := ParseRepoRef("owner/")
	if err == nil {
		t.Fatal("expected error for empty repo")
	}
}

func TestParseRepoRef_SlashInRepo(t *testing.T) {
	ref, err := ParseRepoRef("owner/repo/extra")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Repo != "repo/extra" {
		t.Errorf("expected repo/extra, got %s", ref.Repo)
	}
}

func TestRepoRef_String(t *testing.T) {
	ref := RepoRef{Owner: "owner", Repo: "repo"}
	if ref.String() != "owner/repo" {
		t.Errorf("expected owner/repo, got %s", ref.String())
	}
}
