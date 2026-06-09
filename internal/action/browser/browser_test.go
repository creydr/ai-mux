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
