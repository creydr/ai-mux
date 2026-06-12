# Step 1 GIF: Creating config and starting the daemon
# Run: vhs website/vhs/config-and-start.tape

Output website/recordings/config-and-start.gif

Set Shell zsh
Set FontSize 12
Set Width 900
Set Height 500
Set Theme "Catppuccin Mocha"
Set Padding 10

# Backup real config, swap in example data
Hide
Type "cp ~/.config/ai-mux/config.yaml ~/.config/ai-mux/config.yaml.bak && cp website/vhs/example-config.yaml ~/.config/ai-mux/config.yaml && clear"
Enter
Sleep 500ms
Show

Type "cat ~/.config/ai-mux/config.yaml"
Enter
Sleep 3s

# Start daemon
Type "./bin/ai-mux daemon start --background"
Enter
Sleep 2s

# Check status
Type "./bin/ai-mux daemon status"
Enter
Sleep 2s

Sleep 1s

# Restore real config
Hide
Type "mv ~/.config/ai-mux/config.yaml.bak ~/.config/ai-mux/config.yaml"
Enter
Sleep 500ms
Show
