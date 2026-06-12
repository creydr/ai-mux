# Hero GIF: Full dashboard overview
# Run: vhs website/vhs/dashboard-overview.tape
# Prerequisites: ai-mux daemon running with repos configured

Output website/recordings/dashboard-overview.gif

Set Shell zsh
Set FontSize 12
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set Padding 10

Hide
Type "./bin/ai-mux dashboard"
Enter
Sleep 6s
Show

Sleep 1s

# Navigate issues
Down
Sleep 250ms
Down
Sleep 1s

# Switch to PRs tab
Tab
Sleep 1 s

# switch repo
Left
Sleep 250ms
Down
Sleep 250ms
Down
Sleep 250ms
Down
Sleep 250ms
Down
Sleep 250ms
Down
Sleep 250ms
Down
Sleep 250ms
Enter
Sleep 250ms

# Navigate PRs
Down
Sleep 250ms
Down
Sleep 250ms
Down
Sleep 250ms
Down
Sleep 250ms
Enter # [all...]
Sleep 2s
Up
Sleep 250ms
Up
Sleep 250ms
Up
Sleep 250ms
Up
Sleep 250ms
Enter # open PR
Sleep 1s

# Open Agent
Type "a"
Sleep 250ms
Down
Sleep 250ms
Up
Sleep 250ms
Enter
Sleep 2s

# Worktree choice - new worktree
Enter
Sleep 3.5s

# Prompt
Type "Do a review on this PR"
Sleep 250ms
Enter
Sleep 250ms
Ctrl+b
Sleep 250ms
Type "n"
Sleep 250ms
Type "PR review"
Sleep 250ms
Enter
Sleep 250ms
Ctrl+b
Sleep 250ms
Type "d"
Sleep 1s

# Switch to Sessions tab
Tab
Sleep 1 s

# Resume session
Sleep 250ms
Down
Sleep 250ms
Down
Sleep 250ms
Enter
Sleep 3s
Ctrl+b
Sleep 250ms
Type "d"

Sleep 1s
