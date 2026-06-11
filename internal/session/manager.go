package session

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	agentpkg "github.com/creydr/ai-mux/internal/action/agent"
	"github.com/creydr/ai-mux/internal/config"
	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/worktree"
)

const tmuxPrefix = "ai-mux-"

type StatusCallback func(sess *Session)

type WorktreeAction string

const (
	WorktreeCreate WorktreeAction = ""
	WorktreeReuse  WorktreeAction = "reuse"
	WorktreeFresh  WorktreeAction = "fresh"
	WorktreeNew    WorktreeAction = "new"
)

type WorktreeCreator interface {
	Create(repoPath, name string) (string, error)
	CreateForPR(repoPath, name, repoFullName string, prNumber int) (string, error)
	Remove(repoPath, wtPath string) error
	Exists(repoPath, name string) bool
}

type CommandBuilder interface {
	HasAgent(name string) bool
	GetCommand(agentName string) string
}

type OutputSubscription struct {
	ch     chan []byte
	cancel context.CancelFunc
}

type Manager struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	monitors    map[string]context.CancelFunc
	tmux        TmuxExecutor
	runner      CommandBuilder
	worktrees   WorktreeCreator
	repos       map[string]config.RepoConfig
	store       *Store
	outputDir   string
	maxParallel int
	onStatus    StatusCallback

	outputMu   sync.RWMutex
	outputSubs map[string][]*OutputSubscription
}

type ManagerConfig struct {
	Agents      []config.AgentConfig
	Repos       []config.RepoConfig
	OutputDir   string
	MaxParallel int
	OnStatus    StatusCallback
	Store       *Store
}

func NewManager(cfg ManagerConfig) *Manager {
	if cfg.OutputDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = os.TempDir()
		}
		cfg.OutputDir = filepath.Join(home, ".ai-mux", "sessions")
	}
	if cfg.MaxParallel <= 0 {
		cfg.MaxParallel = 5
	}

	repos := make(map[string]config.RepoConfig, len(cfg.Repos))
	for _, r := range cfg.Repos {
		repos[r.Name] = r
	}

	wm := worktree.NewManager()
	runner := agentpkg.NewRunner(cfg.Agents)

	m := &Manager{
		sessions:    make(map[string]*Session),
		monitors:    make(map[string]context.CancelFunc),
		tmux:        NewTmuxCLI(),
		runner:      runner,
		worktrees:   wm,
		repos:       repos,
		store:       cfg.Store,
		outputDir:   cfg.OutputDir,
		maxParallel: cfg.MaxParallel,
		onStatus:    cfg.OnStatus,
		outputSubs:  make(map[string][]*OutputSubscription),
	}

	if m.store != nil {
		if saved, err := m.store.Load(); err == nil {
			for id, sess := range saved {
				if sess.IsActive() {
					m.sessions[id] = sess
				}
			}
			m.persist()
		}
	}

	return m
}

func (m *Manager) SetTmux(t TmuxExecutor) {
	m.tmux = t
}

func (m *Manager) SetOnStatus(cb StatusCallback) {
	m.onStatus = cb
}

func (m *Manager) SetWorktrees(w WorktreeCreator) {
	m.worktrees = w
}

func (m *Manager) SetRunner(r CommandBuilder) {
	m.runner = r
}

// startMonitor spawns a monitor goroutine for the session if one is not
// already running. The caller MUST hold m.mu.
func (m *Manager) startMonitor(sess *Session) {
	if _, exists := m.monitors[sess.ID]; exists {
		return // already monitored
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.monitors[sess.ID] = cancel
	go m.monitorSession(ctx, sess)
}

// stopMonitor cancels a running monitor goroutine for the given session ID.
// The caller MUST hold m.mu.
func (m *Manager) stopMonitor(sessionID string) {
	if cancel, ok := m.monitors[sessionID]; ok {
		cancel()
		delete(m.monitors, sessionID)
	}
}

func (m *Manager) Spawn(itemRepo string, itemNumber int, itemType string, itemKey string, agentName string, wtAction WorktreeAction, contextPrompt string) (*Session, error) {
	m.mu.Lock()

	active := 0
	for _, s := range m.sessions {
		if s.IsActive() {
			active++
		}
	}
	if active >= m.maxParallel {
		m.mu.Unlock()
		return nil, fmt.Errorf("max parallel sessions reached (%d)", m.maxParallel)
	}

	if !m.runner.HasAgent(agentName) {
		m.mu.Unlock()
		return nil, fmt.Errorf("agent %q not configured", agentName)
	}

	repo, ok := m.repos[itemRepo]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("repo %q not configured", itemRepo)
	}

	var id string
	if itemType == string(provider.ItemTypeJira) {
		id = generateIDForKey("jira", itemKey)
	} else {
		prefix := "fix"
		if itemType == string(provider.ItemTypePR) {
			prefix = "rev"
		}
		id = generateID(prefix, itemNumber)
	}

	sess := &Session{
		ID:            id,
		ItemRepo:      itemRepo,
		ItemNumber:    itemNumber,
		ItemType:      itemType,
		ItemKey:       itemKey,
		Agent:         agentName,
		TmuxSession:   tmuxPrefix + id,
		RepoPath:      repo.Path,
		Status:        StatusPending,
		ContextPrompt: contextPrompt,
		CreatedAt:     time.Now(),
	}

	var wtName string
	if itemType == string(provider.ItemTypeJira) {
		wtName = fmt.Sprintf("jira-%s-%s", sanitizeBranchName(agentName), strings.ToLower(itemKey))
	} else {
		wtName = fmt.Sprintf("%s-%s-%d", itemType, sanitizeBranchName(agentName), itemNumber)
	}
	wtPath, err := m.resolveWorktree(repo.Path, wtName, itemRepo, itemNumber, itemType, wtAction)
	if err != nil {
		m.mu.Unlock()
		return nil, err
	}
	sess.Worktree = wtPath

	cmdStr := m.runner.GetCommand(agentName)

	sessOutputDir := filepath.Join(m.outputDir, id)
	if err := os.MkdirAll(sessOutputDir, 0755); err != nil {
		m.worktrees.Remove(repo.Path, wtPath)
		m.mu.Unlock()
		return nil, fmt.Errorf("creating session output dir: %w", err)
	}

	if err := m.tmux.NewSession(sess.TmuxSession, wtPath, cmdStr); err != nil {
		m.worktrees.Remove(repo.Path, wtPath)
		m.mu.Unlock()
		return nil, fmt.Errorf("starting tmux session: %w", err)
	}

	outputLog := filepath.Join(sessOutputDir, "output.log")
	if err := m.tmux.PipePaneToFile(sess.TmuxSession, outputLog); err != nil {
		m.tmux.KillSession(sess.TmuxSession)
		m.worktrees.Remove(repo.Path, wtPath)
		m.mu.Unlock()
		return nil, fmt.Errorf("setting up output capture: %w", err)
	}

	sess.Status = StatusRunning
	m.sessions[id] = sess

	m.startMonitor(sess)

	m.mu.Unlock()

	m.notifyStatus(sess)
	return sess, nil
}

func (m *Manager) resolveWorktree(repoPath, wtName, itemRepo string, itemNumber int, itemType string, action WorktreeAction) (string, error) {
	switch action {
	case WorktreeReuse:
		return filepath.Join(repoPath, ".worktrees", wtName), nil
	case WorktreeFresh:
		wtPath := filepath.Join(repoPath, ".worktrees", wtName)
		for _, s := range m.sessions {
			if s.Worktree == wtPath && s.IsActive() {
				return "", fmt.Errorf("worktree is in use by active session %s", s.ID)
			}
		}
		m.worktrees.Remove(repoPath, wtPath)
		return m.createWorktree(repoPath, wtName, itemRepo, itemNumber, itemType)
	case WorktreeNew:
		candidate := wtName
		for i := 2; m.worktrees.Exists(repoPath, candidate); i++ {
			candidate = fmt.Sprintf("%s-%d", wtName, i)
		}
		return m.createWorktree(repoPath, candidate, itemRepo, itemNumber, itemType)
	default:
		if m.worktrees.Exists(repoPath, wtName) {
			return "", worktree.ErrWorktreeExists
		}
		return m.createWorktree(repoPath, wtName, itemRepo, itemNumber, itemType)
	}
}

func (m *Manager) createWorktree(repoPath, name, itemRepo string, itemNumber int, itemType string) (string, error) {
	var wtPath string
	var err error
	if itemType == string(provider.ItemTypePR) {
		wtPath, err = m.worktrees.CreateForPR(repoPath, name, itemRepo, itemNumber)
	} else {
		wtPath, err = m.worktrees.Create(repoPath, name)
	}
	if err != nil {
		return "", fmt.Errorf("creating worktree: %w", err)
	}
	return wtPath, nil
}

func (m *Manager) WorktreeExists(itemRepo string, itemNumber int, itemType string, agentName string) bool {
	repo, ok := m.repos[itemRepo]
	if !ok {
		return false
	}
	wtName := fmt.Sprintf("%s-%s-%d", itemType, sanitizeBranchName(agentName), itemNumber)
	return m.worktrees.Exists(repo.Path, wtName)
}

func (m *Manager) WorktreeExistsForKey(itemRepo string, itemKey string, agentName string) bool {
	repo, ok := m.repos[itemRepo]
	if !ok {
		return false
	}
	wtName := fmt.Sprintf("jira-%s-%s", sanitizeBranchName(agentName), strings.ToLower(itemKey))
	return m.worktrees.Exists(repo.Path, wtName)
}

func (m *Manager) FindByWorktree(wtPath string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.sessions {
		if s.Worktree == wtPath && s.IsActive() {
			cp := *s
			return &cp
		}
	}
	return nil
}

func (m *Manager) Stop(sessionID string) error {
	m.mu.Lock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session %q not found", sessionID)
	}
	if !sess.IsActive() {
		m.mu.Unlock()
		return fmt.Errorf("session %q is not active", sessionID)
	}
	tmuxSession := sess.TmuxSession
	sess.Status = StatusStopped
	now := time.Now()
	sess.CompletedAt = &now
	m.stopMonitor(sessionID)
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	m.tmux.SendKeys(tmuxSession, "C-c")
	time.Sleep(500 * time.Millisecond)
	m.tmux.KillSession(tmuxSession)

	m.notifyStatus(sess)
	return nil
}

func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		cp := *s
		result = append(result, &cp)
	}
	return result
}

func (m *Manager) Get(sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}
	cp := *sess
	return &cp, nil
}

func (m *Manager) AttachOutput(sessionID string) (<-chan []byte, func(), error) {
	m.mu.RLock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		m.mu.RUnlock()
		return nil, nil, fmt.Errorf("session %q not found", sessionID)
	}
	active := sess.IsActive()
	tmuxName := sess.TmuxSession
	m.mu.RUnlock()

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan []byte, 64)

	sub := &OutputSubscription{ch: ch, cancel: cancel}

	m.outputMu.Lock()
	m.outputSubs[sessionID] = append(m.outputSubs[sessionID], sub)
	m.outputMu.Unlock()

	if active {
		go func() {
			m.pollCapturePane(ctx, tmuxName, ch)
			close(ch)
		}()
	} else {
		outputLog := filepath.Join(m.outputDir, sessionID, "output.log")
		go func() {
			m.sendFileOnce(ctx, outputLog, ch)
			close(ch)
		}()
	}

	cancelFn := func() {
		cancel()
		m.outputMu.Lock()
		subs := m.outputSubs[sessionID]
		for i, s := range subs {
			if s == sub {
				m.outputSubs[sessionID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		m.outputMu.Unlock()
	}

	return ch, cancelFn, nil
}

func (m *Manager) SendInput(sessionID, input string) error {
	m.mu.RLock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("session %q not found", sessionID)
	}
	if !sess.IsActive() {
		m.mu.RUnlock()
		return fmt.Errorf("session %q is not active", sessionID)
	}
	tmuxName := sess.TmuxSession
	m.mu.RUnlock()

	return m.tmux.SendKeys(tmuxName, input)
}

func (m *Manager) TypeInput(sessionID, text string) error {
	m.mu.RLock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("session %q not found", sessionID)
	}
	if !sess.IsActive() {
		m.mu.RUnlock()
		return fmt.Errorf("session %q is not active", sessionID)
	}
	tmuxName := sess.TmuxSession
	m.mu.RUnlock()

	return m.tmux.TypeKeys(tmuxName, text)
}

func (m *Manager) Reconcile() error {
	names, err := m.tmux.ListSessions(tmuxPrefix)
	if err != nil {
		return fmt.Errorf("listing tmux sessions: %w", err)
	}

	live := make(map[string]bool, len(names))
	for _, name := range names {
		live[strings.TrimPrefix(name, tmuxPrefix)] = true
	}

	m.mu.Lock()

	for id, sess := range m.sessions {
		if !sess.IsActive() {
			continue
		}
		if live[id] {
			m.startMonitor(sess)
			delete(live, id)
		} else {
			now := time.Now()
			sess.Status = StatusCompleted
			sess.CompletedAt = &now
		}
	}

	for id := range live {
		name := tmuxPrefix + id
		sess := &Session{
			ID:          id,
			Name:        "(recovered)",
			TmuxSession: name,
			Status:      StatusRunning,
			CreatedAt:   time.Now(),
		}
		log.Printf("recovered orphaned tmux session %s with limited metadata", name)
		m.sessions[id] = sess
		m.startMonitor(sess)
	}

	m.mu.Unlock()

	m.persist()
	return nil
}

func (m *Manager) Rename(sessionID, name string) error {
	m.mu.Lock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session %q not found", sessionID)
	}
	sess.Name = name
	m.mu.Unlock()

	m.persist()
	return nil
}

func (m *Manager) FindByItem(repo string, number int) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.sessions {
		if s.ItemRepo == repo && s.ItemNumber == number && s.IsActive() {
			cp := *s
			return &cp
		}
	}
	return nil
}

func (m *Manager) FindByItemKey(key string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.sessions {
		if s.ItemKey == key && s.IsActive() {
			cp := *s
			return &cp
		}
	}
	return nil
}

func (m *Manager) monitorSession(ctx context.Context, sess *Session) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		m.mu.RLock()
		if !sess.IsActive() {
			m.mu.RUnlock()
			return
		}
		tmuxName := sess.TmuxSession
		sessID := sess.ID
		m.mu.RUnlock()

		if m.tmux.IsPaneDead(tmuxName) || !m.tmux.HasSession(tmuxName) {
			m.saveFinalScreen(tmuxName, sessID)
			m.tmux.KillSession(tmuxName)

			m.mu.Lock()
			now := time.Now()
			sess.CompletedAt = &now

			exitCode := m.detectExitCode(sessID)
			if exitCode != nil {
				sess.ExitCode = exitCode
				if *exitCode == 0 {
					sess.Status = StatusCompleted
				} else {
					sess.Status = StatusFailed
					sess.Error = fmt.Sprintf("exit code %d", *exitCode)
				}
			} else {
				sess.Status = StatusCompleted
			}
			delete(m.sessions, sessID)
			delete(m.monitors, sessID)
			m.mu.Unlock()

			m.notifyStatus(sess)
			return
		}

		waitingInput := m.checkWaitingInput(tmuxName)
		m.mu.Lock()
		changed := sess.WaitingInput != waitingInput
		if changed {
			sess.WaitingInput = waitingInput
		}
		m.mu.Unlock()
		if changed {
			m.notifyStatus(sess)
		}
	}
}

func (m *Manager) checkWaitingInput(tmuxName string) bool {
	pid, err := m.tmux.PanePID(tmuxName)
	if err != nil {
		return false
	}

	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return false
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return false
	}
	return fields[2] == "S"
}

func (m *Manager) detectExitCode(sessionID string) *int {
	exitFile := filepath.Join(m.outputDir, sessionID, "exit_code")
	data, err := os.ReadFile(exitFile)
	if err != nil {
		return nil
	}
	code, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return nil
	}
	return &code
}

func (m *Manager) notifyStatus(sess *Session) {
	m.persist()
	if m.onStatus != nil {
		cp := *sess
		m.onStatus(&cp)
	}
}

// persist snapshots the sessions map under a read lock and saves to the store.
// Must NOT be called while m.mu is held.
func (m *Manager) persist() {
	if m.store == nil {
		return
	}
	m.mu.RLock()
	snapshot := make(map[string]*Session, len(m.sessions))
	for k, v := range m.sessions {
		cp := *v
		snapshot[k] = &cp
	}
	m.mu.RUnlock()
	m.store.Save(snapshot)
}

func (m *Manager) saveFinalScreen(tmuxName, sessionID string) {
	output, err := m.tmux.CapturePane(tmuxName)
	if err != nil {
		log.Printf("failed to capture pane for session %s: %v", sessionID, err)
		return
	}
	screenFile := filepath.Join(m.outputDir, sessionID, "screen.txt")
	if err := os.WriteFile(screenFile, []byte(output), 0644); err != nil {
		log.Printf("failed to save final screen for session %s: %v", sessionID, err)
	}
}

func (m *Manager) pollCapturePane(ctx context.Context, tmuxName string, ch chan<- []byte) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			output, err := m.tmux.CapturePane(tmuxName)
			if err != nil {
				continue
			}
			select {
			case ch <- []byte(output):
			case <-ctx.Done():
				return
			}
		}
	}
}

func (m *Manager) sendFileOnce(ctx context.Context, path string, ch chan<- []byte) {
	screenPath := filepath.Join(filepath.Dir(path), "screen.txt")
	data, err := os.ReadFile(screenPath)
	if err != nil {
		data, err = os.ReadFile(path)
		if err != nil {
			return
		}
		data = stripANSI(data)
	}
	select {
	case ch <- data:
	case <-ctx.Done():
	}
}

var branchUnsafe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitizeBranchName(s string) string {
	return strings.Trim(branchUnsafe.ReplaceAllString(s, "-"), "-")
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b\[[\?]?[0-9;]*[a-zA-Z]`)

func stripANSI(data []byte) []byte {
	return ansiRe.ReplaceAll(data, nil)
}
