package attach

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/creydr/ai-mux/internal/provider"
)

type Ref struct {
	Type   provider.ItemType
	Owner  string
	Repo   string
	Number int
}

func ParseRef(s string) (Ref, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 4 {
		return Ref{}, fmt.Errorf("invalid ref %q: expected type/owner/repo/number", s)
	}

	var itemType provider.ItemType
	switch parts[0] {
	case "issue", "issues":
		itemType = provider.ItemTypeIssue
	case "pr", "prs", "pull":
		itemType = provider.ItemTypePR
	default:
		return Ref{}, fmt.Errorf("invalid item type %q: expected issue or pr", parts[0])
	}

	if parts[1] == "" || parts[2] == "" {
		return Ref{}, fmt.Errorf("invalid ref %q: owner and repo cannot be empty", s)
	}

	number, err := strconv.Atoi(parts[3])
	if err != nil {
		return Ref{}, fmt.Errorf("invalid number %q in ref: %w", parts[3], err)
	}

	return Ref{
		Type:   itemType,
		Owner:  parts[1],
		Repo:   parts[2],
		Number: number,
	}, nil
}

func (r Ref) RepoRef() provider.RepoRef {
	return provider.RepoRef{Owner: r.Owner, Repo: r.Repo}
}
