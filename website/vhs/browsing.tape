# Step 2 GIF: Browsing dashboard with search and tabs
# Run: vhs website/vhs/browsing.tape
# Prerequisites: ai-mux daemon running with repos configured

Output website/recordings/browsing.gif

Set Shell zsh
Set FontSize 12
Set Width 900
Set Height 500
Set Theme "Catppuccin Mocha"
Set Padding 10

Hide
Type "./bin/ai-mux dashboard"
Enter
Sleep 6s
Show

Sleep 1s

# Navigate issues list
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Enter # click on "more..."
Sleep 2s
Down
Sleep 300ms
Down
Sleep 300ms

# Navigate via repo overview
Left
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Enter # select mcplo
Sleep 300ms
Right
Sleep 1s

# Open "all..."
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Enter # click "all..."
Sleep 3s

# Open search with :
Type ":"
Sleep 300ms
Type "helm chart"
Sleep 300ms
Enter
Sleep 300ms

Down
Sleep 300ms
Down
Sleep 300ms
Enter
Sleep 2s
Escape
Sleep 300ms

# Switch to PRs tab
Tab
Sleep 1s

# Open "all" view
Left
Sleep 300ms
Up
Sleep 300ms
Up
Sleep 300ms
Up
Sleep 300ms
Up
Sleep 300ms
Enter
Sleep 1s
Right
Sleep 300ms

# Open detail
Enter
Sleep 2s

# Scroll content
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms

# Back
Escape
Sleep 1s

# Go to sessions
Tab
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Enter
Sleep 2s
# no idea how to exit a tmux session here (ctrl+b d)
