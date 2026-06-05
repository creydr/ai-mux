package browser

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/creydr/ai-mux/internal/action"
)

type Action struct{}

func New() *Action {
	return &Action{}
}

func (a *Action) Type() action.ActionType {
	return action.ActionOpenBrowser
}

func (a *Action) Name() string {
	return "Open in Browser"
}

func (a *Action) Execute(_ context.Context, actCtx action.Context) action.Result {
	if actCtx.Item == nil || actCtx.Item.URL == "" {
		return action.Result{Error: fmt.Errorf("item has no URL")}
	}
	cmd := OpenCommand(actCtx.Item.URL)
	if err := cmd.Run(); err != nil {
		return action.Result{Error: fmt.Errorf("opening browser: %w", err)}
	}
	return action.Result{
		Success: true,
		Message: fmt.Sprintf("Opened %s in browser", actCtx.Item.URL),
	}
}

func OpenCommand(url string) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url)
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return exec.Command("xdg-open", url)
	}
}
