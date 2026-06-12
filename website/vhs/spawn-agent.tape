# Step 3 GIF: Spawning an AI agent from an issue
# Run: vhs website/vhs/spawn-agent.tape
# Prerequisites: ai-mux daemon running with repos configured, agents configured

Output website/recordings/spawn-agent.gif

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

# Select an issue
Down
Sleep 500ms
Down
Sleep 500ms

# Press 'a' to spawn agent
Type "a"
Sleep 1.5s

# Agent picker appears - select agent
Down
Up
Enter
Sleep 1.5s

# Worktree choice - new worktree
Enter
Sleep 2s

# Switch to Sessions tab to see the session
Tab
Sleep 500ms
Tab
Sleep 500ms
Tab
Sleep 1.5s

# Attach to session
Enter
Sleep 3s

Sleep 1s
