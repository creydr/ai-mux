package commands

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommand_SubcommandRegistration(t *testing.T) {
	names := make(map[string]bool)
	for _, cmd := range rootCmd.Commands() {
		names[cmd.Name()] = true
	}

	for _, want := range []string{"version", "daemon", "dashboard", "session"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestRootCommand_ConfigFlag(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("config")
	if f == nil {
		t.Fatal("missing --config flag")
	}
	if f.DefValue != "" {
		t.Errorf("--config default = %q, want empty", f.DefValue)
	}
}

func TestVersionCommand(t *testing.T) {
	Version = "test-1.2.3"

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "test-1.2.3") {
		t.Errorf("version output = %q, want it to contain %q", got, "test-1.2.3")
	}
}

func TestDaemonCommand_SubcommandRegistration(t *testing.T) {
	names := make(map[string]bool)
	for _, cmd := range daemonCmd.Commands() {
		names[cmd.Name()] = true
	}

	for _, want := range []string{"start", "stop", "status"} {
		if !names[want] {
			t.Errorf("daemon missing subcommand %q", want)
		}
	}
}

func TestDaemonStartCommand_ForegroundFlag(t *testing.T) {
	f := daemonStartCmd.Flags().Lookup("foreground")
	if f == nil {
		t.Fatal("missing --foreground flag on daemon start")
	}
	if f.DefValue != "false" {
		t.Errorf("--foreground default = %q, want %q", f.DefValue, "false")
	}
}

func TestSessionCommand_SubcommandRegistration(t *testing.T) {
	names := make(map[string]bool)
	for _, cmd := range sessionCmd.Commands() {
		names[cmd.Name()] = true
	}

	for _, want := range []string{"list", "attach"} {
		if !names[want] {
			t.Errorf("session missing subcommand %q", want)
		}
	}
}

func TestSessionAttachCommand_RequiresExactlyOneArg(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"session", "attach"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("session attach with no args should fail")
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	cfgPath = "/nonexistent/path/config.yaml"
	defer func() { cfgPath = "" }()

	_, err := loadConfig()
	if err == nil {
		t.Fatal("loadConfig should fail with missing config file")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("error = %q, want it to mention loading config", err.Error())
	}
}
