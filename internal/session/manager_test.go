package session

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/creydr/ai-mux/internal/config"
)

type mockTmux struct {
	mu       sync.Mutex
	sessions map[string]bool
	keys     []string
	pipes    []string
	panePIDs map[string]int
}

func newMockTmux() *mockTmux {
	return &mockTmux{
		sessions: make(map[string]bool),
		panePIDs: make(map[string]int),
	}
}

func (m *mockTmux) NewSession(name, workdir, command string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[name] = true
	return nil
}

func (m *mockTmux) KillSession(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, name)
	return nil
}

func (m *mockTmux) SendKeys(name, keys string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.sessions[name] {
		return fmt.Errorf("session %q not found", name)
	}
	m.keys = append(m.keys, keys)
	return nil
}

func (m *mockTmux) TypeKeys(name, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.sessions[name] {
		return fmt.Errorf("session %q not found", name)
	}
	m.keys = append(m.keys, text)
	return nil
}

func (m *mockTmux) ListSessions(prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []string
	for name := range m.sessions {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			result = append(result, name)
		}
	}
	return result, nil
}

func (m *mockTmux) HasSession(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[name]
}

func (m *mockTmux) IsPaneDead(name string) bool {
	return false
}

func (m *mockTmux) PanePID(name string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pid, ok := m.panePIDs[name]
	if !ok {
		return 0, fmt.Errorf("no pane pid for %q", name)
	}
	return pid, nil
}

func (m *mockTmux) PipePaneToFile(name, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pipes = append(m.pipes, name+":"+path)
	return nil
}

func (m *mockTmux) CapturePane(name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.sessions[name] {
		return "", fmt.Errorf("session %q not found", name)
	}
	return "mock output", nil
}

type mockWorktrees struct {
	mu      sync.Mutex
	created map[string]string
	removed []string
}

func newMockWorktrees() *mockWorktrees {
	return &mockWorktrees{created: make(map[string]string)}
}

func (m *mockWorktrees) Create(repoPath, name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	path := repoPath + "/.worktrees/" + name
	m.created[name] = path
	return path, nil
}

func (m *mockWorktrees) CreateForPR(repoPath, name, repoFullName string, prNumber int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	path := repoPath + "/.worktrees/" + name
	m.created[name] = path
	return path, nil
}

func (m *mockWorktrees) Exists(repoPath, name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.created[name]
	return ok
}

func (m *mockWorktrees) Remove(repoPath, wtPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removed = append(m.removed, wtPath)
	return nil
}

type mockRunner struct{}

func (m *mockRunner) HasAgent(name string) bool {
	return name == "claude"
}

func (m *mockRunner) GetCommand(agentName string) string {
	return "echo mock-agent"
}

func testManager(t *testing.T) (*Manager, *mockTmux) {
	t.Helper()

	mock := newMockTmux()
	mgr := NewManager(ManagerConfig{
		Agents: []config.AgentConfig{
			{
				Name:    "claude",
				Command: "claude",
			},
		},
		Repos: []config.RepoConfig{
			{Name: "owner/repo", Path: "/tmp/test-repo"},
		},
		OutputDir:   t.TempDir(),
		MaxParallel: 3,
	})
	mgr.SetTmux(mock)
	mgr.SetWorktrees(newMockWorktrees())
	mgr.SetRunner(&mockRunner{})

	return mgr, mock
}

func TestManager_Spawn(t *testing.T) {
	mgr, mock := testManager(t)

	sess, err := mgr.Spawn("owner/repo", 42, "issue", "", "claude", "", "")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if sess.ID == "" {
		t.Error("session ID should not be empty")
	}
	if sess.ItemRepo != "owner/repo" {
		t.Errorf("ItemRepo = %q, want %q", sess.ItemRepo, "owner/repo")
	}
	if sess.ItemNumber != 42 {
		t.Errorf("ItemNumber = %d, want 42", sess.ItemNumber)
	}
	if sess.Agent != "claude" {
		t.Errorf("Agent = %q, want %q", sess.Agent, "claude")
	}
	if sess.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", sess.Status, StatusRunning)
	}

	mock.mu.Lock()
	if !mock.sessions[sess.TmuxSession] {
		t.Error("tmux session should exist")
	}
	if len(mock.pipes) == 0 {
		t.Error("pipe-pane should have been called")
	}
	mock.mu.Unlock()
}

func TestManager_Spawn_UnknownAgent(t *testing.T) {
	mgr, _ := testManager(t)

	_, err := mgr.Spawn("owner/repo", 42, "issue", "", "unknown", "", "")
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestManager_Spawn_UnknownRepo(t *testing.T) {
	mgr, _ := testManager(t)

	_, err := mgr.Spawn("other/repo", 42, "issue", "", "claude", "", "")
	if err == nil {
		t.Fatal("expected error for unknown repo")
	}
}

func TestManager_Spawn_MaxParallel(t *testing.T) {
	mgr, _ := testManager(t)

	for i := 0; i < 3; i++ {
		_, err := mgr.Spawn("owner/repo", i+1, "issue", "", "claude", "", "")
		if err != nil {
			t.Fatalf("Spawn %d failed: %v", i+1, err)
		}
	}

	_, err := mgr.Spawn("owner/repo", 100, "issue", "", "claude", "", "")
	if err == nil {
		t.Fatal("expected error for exceeding max parallel")
	}
}

func TestManager_Stop(t *testing.T) {
	mgr, mock := testManager(t)

	sess, err := mgr.Spawn("owner/repo", 42, "issue", "", "claude", "", "")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if err := mgr.Stop(sess.ID); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	_, err = mgr.Get(sess.ID)
	if err == nil {
		t.Error("stopped session should be removed from manager")
	}

	sessions := mgr.List()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after stop, got %d", len(sessions))
	}

	mock.mu.Lock()
	if mock.sessions[sess.TmuxSession] {
		t.Error("tmux session should have been killed")
	}
	mock.mu.Unlock()
}

func TestManager_Stop_NotFound(t *testing.T) {
	mgr, _ := testManager(t)

	err := mgr.Stop("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestManager_List(t *testing.T) {
	mgr, _ := testManager(t)

	_, err := mgr.Spawn("owner/repo", 1, "issue", "", "claude", "", "")
	if err != nil {
		t.Fatalf("Spawn 1 failed: %v", err)
	}
	_, err = mgr.Spawn("owner/repo", 2, "pr", "", "claude", "", "")
	if err != nil {
		t.Fatalf("Spawn 2 failed: %v", err)
	}

	sessions := mgr.List()
	if len(sessions) != 2 {
		t.Errorf("List returned %d sessions, want 2", len(sessions))
	}
}

func TestManager_Get(t *testing.T) {
	mgr, _ := testManager(t)

	sess, err := mgr.Spawn("owner/repo", 42, "issue", "", "claude", "", "")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	got, err := mgr.Get(sess.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.ID != sess.ID {
		t.Errorf("ID = %q, want %q", got.ID, sess.ID)
	}
	if got.ItemNumber != 42 {
		t.Errorf("ItemNumber = %d, want 42", got.ItemNumber)
	}
}

func TestManager_Get_NotFound(t *testing.T) {
	mgr, _ := testManager(t)

	_, err := mgr.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestManager_SendInput(t *testing.T) {
	mgr, mock := testManager(t)

	sess, err := mgr.Spawn("owner/repo", 42, "issue", "", "claude", "", "")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if err := mgr.SendInput(sess.ID, "hello"); err != nil {
		t.Fatalf("SendInput failed: %v", err)
	}

	mock.mu.Lock()
	if len(mock.keys) == 0 || mock.keys[len(mock.keys)-1] != "hello" {
		t.Errorf("expected send-keys with 'hello', got %v", mock.keys)
	}
	mock.mu.Unlock()
}

func TestManager_SendInput_NotActive(t *testing.T) {
	mgr, _ := testManager(t)

	sess, err := mgr.Spawn("owner/repo", 42, "issue", "", "claude", "", "")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	mgr.Stop(sess.ID)

	err = mgr.SendInput(sess.ID, "hello")
	if err == nil {
		t.Fatal("expected error for stopped session")
	}
}

func TestManager_FindByItem(t *testing.T) {
	mgr, _ := testManager(t)

	_, err := mgr.Spawn("owner/repo", 42, "issue", "", "claude", "", "")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	found := mgr.FindByItem("owner/repo", 42)
	if found == nil {
		t.Fatal("expected to find session for owner/repo#42")
	}
	if found.ItemNumber != 42 {
		t.Errorf("ItemNumber = %d, want 42", found.ItemNumber)
	}

	notFound := mgr.FindByItem("owner/repo", 99)
	if notFound != nil {
		t.Error("should not find session for owner/repo#99")
	}
}

func TestManager_Reconcile(t *testing.T) {
	mgr, mock := testManager(t)

	mock.mu.Lock()
	mock.sessions["ai-mux-fix-99-deadbeef"] = true
	mock.sessions["ai-mux-rev-50-cafebabe"] = true
	mock.sessions["unrelated-session"] = true
	mock.mu.Unlock()

	if err := mgr.Reconcile(); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	sessions := mgr.List()
	if len(sessions) != 2 {
		t.Errorf("expected 2 reconciled sessions, got %d", len(sessions))
	}
}

func TestManager_StatusCallback(t *testing.T) {
	var statusUpdates []*Session
	var mu sync.Mutex

	mock := newMockTmux()
	mgr := NewManager(ManagerConfig{
		Agents: []config.AgentConfig{
			{
				Name:    "claude",
				Command: "claude",
			},
		},
		Repos: []config.RepoConfig{
			{Name: "owner/repo", Path: "/tmp/test-repo"},
		},
		OutputDir:   t.TempDir(),
		MaxParallel: 5,
		OnStatus: func(sess *Session) {
			mu.Lock()
			statusUpdates = append(statusUpdates, sess)
			mu.Unlock()
		},
	})
	mgr.SetTmux(mock)
	mgr.SetWorktrees(newMockWorktrees())
	mgr.SetRunner(&mockRunner{})

	sess, err := mgr.Spawn("owner/repo", 42, "issue", "", "claude", "", "")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(statusUpdates) == 0 {
		t.Error("expected at least one status callback")
	}
	if statusUpdates[0].Status != StatusRunning {
		t.Errorf("first status = %q, want %q", statusUpdates[0].Status, StatusRunning)
	}
	mu.Unlock()

	mgr.Stop(sess.ID)

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	lastStatus := statusUpdates[len(statusUpdates)-1]
	if lastStatus.Status != StatusStopped {
		t.Errorf("last status = %q, want %q", lastStatus.Status, StatusStopped)
	}
	mu.Unlock()
}

func TestSession_IsActive(t *testing.T) {
	tests := []struct {
		status Status
		want   bool
	}{
		{StatusPending, true},
		{StatusRunning, true},
		{StatusCompleted, false},
		{StatusFailed, false},
		{StatusStopped, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			s := &Session{Status: tt.status}
			if got := s.IsActive(); got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManager_PersistAndRestore(t *testing.T) {
	dir := t.TempDir()
	st := NewStore(dir)

	mock := newMockTmux()
	mgr := NewManager(ManagerConfig{
		Agents: []config.AgentConfig{
			{Name: "claude", Command: "claude"},
		},
		Repos:       []config.RepoConfig{{Name: "owner/repo", Path: "/tmp/test-repo"}},
		OutputDir:   t.TempDir(),
		MaxParallel: 5,
		Store:       st,
	})
	mgr.SetTmux(mock)
	mgr.SetWorktrees(newMockWorktrees())
	mgr.SetRunner(&mockRunner{})

	sess, err := mgr.Spawn("owner/repo", 42, "issue", "", "claude", "", "")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	mgr2 := NewManager(ManagerConfig{
		Agents: []config.AgentConfig{
			{Name: "claude", Command: "claude"},
		},
		Repos:       []config.RepoConfig{{Name: "owner/repo", Path: "/tmp/test-repo"}},
		OutputDir:   t.TempDir(),
		MaxParallel: 5,
		Store:       st,
	})
	mgr2.SetTmux(mock)
	mgr2.SetWorktrees(newMockWorktrees())
	mgr2.SetRunner(&mockRunner{})

	sessions := mgr2.List()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 restored session, got %d", len(sessions))
	}
	if sessions[0].ID != sess.ID {
		t.Errorf("restored ID = %q, want %q", sessions[0].ID, sess.ID)
	}
	if sessions[0].ItemRepo != "owner/repo" {
		t.Errorf("restored ItemRepo = %q, want %q", sessions[0].ItemRepo, "owner/repo")
	}
	if sessions[0].ItemNumber != 42 {
		t.Errorf("restored ItemNumber = %d, want 42", sessions[0].ItemNumber)
	}
	if sessions[0].Agent != "claude" {
		t.Errorf("restored Agent = %q, want %q", sessions[0].Agent, "claude")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID("fix", 42)
	id2 := generateID("fix", 42)

	if id1 == "" {
		t.Error("ID should not be empty")
	}
	if id1 == id2 {
		t.Error("IDs should be unique")
	}
	if len(id1) < 8 {
		t.Errorf("ID too short: %q", id1)
	}
}

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"claude", "claude"},
		{"my agent", "my-agent"},
		{"agent/v2", "agent-v2"},
		{"--leading--", "leading"},
		{"special!@#chars", "special-chars"},
		{"a..b", "a..b"},
		{"hello_world-1.0", "hello_world-1.0"},
		{"claude (YOLO)", "claude-YOLO"},
		{"  spaces  ", "spaces"},
		{"a", "a"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeBranchName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeBranchName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello world", "hello world"},
		{"color code", "\x1b[31mred\x1b[0m", "red"},
		{"bold", "\x1b[1mbold\x1b[0m text", "bold text"},
		{"cursor move", "\x1b[2Aup", "up"},
		{"osc sequence", "\x1b]0;title\x07rest", "rest"},
		{"mixed", "\x1b[1mbold\x1b[0m and \x1b[32mgreen\x1b[0m", "bold and green"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(stripANSI([]byte(tt.input)))
			if got != tt.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestManager_Rename(t *testing.T) {
	mgr, _ := testManager(t)
	sess, err := mgr.Spawn("owner/repo", 42, "issue", "", "claude", "", "")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if err := mgr.Rename(sess.ID, "my-session"); err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	got, err := mgr.Get(sess.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "my-session" {
		t.Errorf("Name = %q, want %q", got.Name, "my-session")
	}
}

func TestManager_Rename_NotFound(t *testing.T) {
	mgr, _ := testManager(t)
	err := mgr.Rename("nonexistent", "name")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestManager_WorktreeExists(t *testing.T) {
	mgr, _ := testManager(t)

	if mgr.WorktreeExists("owner/repo", 42, "issue", "claude") {
		t.Error("worktree should not exist before spawn")
	}

	_, err := mgr.Spawn("owner/repo", 42, "issue", "", "claude", "", "")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if !mgr.WorktreeExists("owner/repo", 42, "issue", "claude") {
		t.Error("worktree should exist after spawn")
	}

	if mgr.WorktreeExists("unknown/repo", 42, "issue", "claude") {
		t.Error("worktree should not exist for unknown repo")
	}
}

func TestManager_FindByWorktree(t *testing.T) {
	mgr, _ := testManager(t)

	sess, err := mgr.Spawn("owner/repo", 42, "issue", "", "claude", "", "")
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	found := mgr.FindByWorktree(sess.Worktree)
	if found == nil {
		t.Fatal("should find session by worktree path")
	}
	if found.ID != sess.ID {
		t.Errorf("found ID = %q, want %q", found.ID, sess.ID)
	}

	notFound := mgr.FindByWorktree("/nonexistent/path")
	if notFound != nil {
		t.Error("should not find session for unknown worktree path")
	}
}
