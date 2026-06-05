package browser

import (
	"runtime"
	"testing"
)

func TestOpenCommand(t *testing.T) {
	cmd := OpenCommand("https://github.com/owner/repo/issues/1")

	switch runtime.GOOS {
	case "darwin":
		if cmd.Path == "" || cmd.Args[0] != "open" {
			t.Errorf("expected 'open' command on darwin, got %v", cmd.Args)
		}
	case "windows":
		if cmd.Args[0] != "rundll32" {
			t.Errorf("expected 'rundll32' command on windows, got %v", cmd.Args)
		}
	default:
		if cmd.Args[len(cmd.Args)-1] != "https://github.com/owner/repo/issues/1" {
			t.Errorf("expected URL as last arg, got %v", cmd.Args)
		}
	}
}

func TestAction_Name(t *testing.T) {
	a := New()
	if a.Name() != "Open in Browser" {
		t.Errorf("expected 'Open in Browser', got %q", a.Name())
	}
}

func TestAction_Type(t *testing.T) {
	a := New()
	if a.Type() != "open_browser" {
		t.Errorf("expected 'open_browser', got %q", a.Type())
	}
}
