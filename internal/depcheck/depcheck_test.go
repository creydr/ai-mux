package depcheck

import (
	"strings"
	"testing"
)

func TestCheck_AllPresent(t *testing.T) {
	deps := []Dependency{
		{Name: "go", Reason: "test"},
		{Name: "git", Reason: "test"},
	}
	if err := Check(deps); err != nil {
		t.Fatalf("expected no error for installed tools, got: %v", err)
	}
}

func TestCheck_Missing(t *testing.T) {
	deps := []Dependency{
		{Name: "go", Reason: "test"},
		{Name: "nonexistent-tool-xyz-12345", Reason: "needed for testing"},
	}
	err := Check(deps)
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
	if !strings.Contains(err.Error(), "nonexistent-tool-xyz-12345") {
		t.Fatalf("error should mention the missing tool, got: %v", err)
	}
	if !strings.Contains(err.Error(), "needed for testing") {
		t.Fatalf("error should mention the reason, got: %v", err)
	}
}

func TestCheck_Empty(t *testing.T) {
	if err := Check(nil); err != nil {
		t.Fatalf("expected no error for empty deps, got: %v", err)
	}
}
