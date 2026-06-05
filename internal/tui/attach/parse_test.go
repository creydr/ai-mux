package attach

import (
	"testing"

	"github.com/creydr/ai-mux/internal/provider"
)

func TestParseRef_Issue(t *testing.T) {
	ref, err := ParseRef("issue/owner/repo/42")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Type != provider.ItemTypeIssue {
		t.Errorf("expected issue, got %s", ref.Type)
	}
	if ref.Owner != "owner" || ref.Repo != "repo" {
		t.Errorf("unexpected owner/repo: %s/%s", ref.Owner, ref.Repo)
	}
	if ref.Number != 42 {
		t.Errorf("expected 42, got %d", ref.Number)
	}
}

func TestParseRef_PR(t *testing.T) {
	ref, err := ParseRef("pr/org/project/10")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Type != provider.ItemTypePR {
		t.Errorf("expected pr, got %s", ref.Type)
	}
	if ref.Number != 10 {
		t.Errorf("expected 10, got %d", ref.Number)
	}
}

func TestParseRef_PullAlias(t *testing.T) {
	ref, err := ParseRef("pull/owner/repo/5")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Type != provider.ItemTypePR {
		t.Errorf("expected pr type for pull alias, got %s", ref.Type)
	}
}

func TestParseRef_IssuesAlias(t *testing.T) {
	ref, err := ParseRef("issues/owner/repo/1")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Type != provider.ItemTypeIssue {
		t.Errorf("expected issue, got %s", ref.Type)
	}
}

func TestParseRef_InvalidFormat(t *testing.T) {
	cases := []string{
		"",
		"owner/repo/1",
		"pr/owner/repo",
		"pr/owner/repo/abc",
		"unknown/owner/repo/1",
		"pr//repo/1",
		"pr/owner//1",
	}
	for _, c := range cases {
		_, err := ParseRef(c)
		if err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestRef_RepoRef(t *testing.T) {
	ref := Ref{Type: provider.ItemTypePR, Owner: "o", Repo: "r", Number: 1}
	rr := ref.RepoRef()
	if rr.Owner != "o" || rr.Repo != "r" {
		t.Errorf("unexpected repo ref: %v", rr)
	}
}
