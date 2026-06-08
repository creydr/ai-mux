package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	agentpkg "github.com/creydr/ai-mux/internal/action/agent"
	"github.com/creydr/ai-mux/internal/config"
	"github.com/creydr/ai-mux/internal/worktree"
)

const tmuxPrefix = "ai-mux-"

type StatusCallback func(sess *Session)

type WorktreeCreator interface {
	Create(repoPath, name string) (string, error)
	CreateForPR(repoPath, name, repoFullName string, prNumber int) (string, error)
	Remove(repoPath, wtPath string) error
}

type CommandBuilder interface {
	HasAgent(name string) bool
	GetCommand(agentName string) string
	GetPostSession(agentName string) string
}

type OutputSubscription struct {
	ch     chan []byte
	cancel context.CancelFunc
}

type Manager struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	tmux        TmuxExecutor
	runner      CommandBuilder
	worktrees   WorktreeCreator
	postHandler func(repoPath, wtPath, postSession string)
	repos       map[string]config.RepoConfig
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
}

func NewManager(cfg ManagerConfig) *Manager {
	if cfg.OutputDir == "" {
		home, _ := os.UserHomeDir()
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

	return &Manager{
		sessions:  make(map[string]*Session),
		tmux:      NewTmuxCLI(),
		runner:    runner,
		worktrees: wm,
		postHandler: func(repoPath, wtPath, postSession string) {
			handler := worktree.NewPostSessionHandler(postSession, wm)
			handler.Handle(repoPath, wtPath, "")
		},
		repos:       repos,
		outputDir:   cfg.OutputDir,
		maxParallel: cfg.MaxParallel,
		onStatus:    cfg.OnStatus,
		outputSubs:  make(map[string][]*OutputSubscription),
	}
}

func (m *Manager) SetTmux(t TmuxExecutor) {
	m.tmux = t
}

func (m *Manager) SetWorktrees(w WorktreeCreator) {
	m.worktrees = w
}

func (m *Manager) SetRunner(r CommandBuilder) {
	m.runner = r
}

func (m *Manager) SetPostHandler(fn func(repoPath, wtPath, postSession string)) {
	m.postHandler = fn
}

func (m *Manager) Spawn(itemRepo string, itemNumber int, itemType string, agentName string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	active := 0
	for _, s := range m.sessions {
		if s.IsActive() {
			active++
		}
	}
	if active >= m.maxParallel {
		return nil, fmt.Errorf("max parallel sessions reached (%d)", m.maxParallel)
	}

	if !m.runner.HasAgent(agentName) {
		return nil, fmt.Errorf("agent %q not configured", agentName)
	}

	repo, ok := m.repos[itemRepo]
	if !ok {
		return nil, fmt.Errorf("repo %q not configured", itemRepo)
	}

	prefix := "fix"
	if itemType == "pr" {
		prefix = "rev"
	}
	id := generateID(prefix, itemNumber)

	sess := &Session{
		ID:          id,
		ItemRepo:    itemRepo,
		ItemNumber:  itemNumber,
		ItemType:    itemType,
		Agent:       agentName,
		TmuxSession: tmuxPrefix + id,
		RepoPath:    repo.Path,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
	}

	wtName := fmt.Sprintf("%s-%s-%d", itemType, agentName, itemNumber)
	var wtPath string
	var err error
	if itemType == "pr" {
		wtPath, err = m.worktrees.CreateForPR(repo.Path, wtName, itemRepo, itemNumber)
	} else {
		wtPath, err = m.worktrees.Create(repo.Path, wtName)
	}
	if err != nil {
		return nil, fmt.Errorf("creating worktree: %w", err)
	}
	sess.Worktree = wtPath

	cmdStr := m.runner.GetCommand(agentName)

	sessOutputDir := filepath.Join(m.outputDir, id)
	if err := os.MkdirAll(sessOutputDir, 0755); err != nil {
		m.worktrees.Remove(repo.Path, wtPath)
		return nil, fmt.Errorf("creating session output dir: %w", err)
	}

	if err := m.tmux.NewSession(sess.TmuxSession, wtPath, cmdStr); err != nil {
		m.worktrees.Remove(repo.Path, wtPath)
		return nil, fmt.Errorf("starting tmux session: %w", err)
	}

	outputLog := filepath.Join(sessOutputDir, "output.log")
	if err := m.tmux.PipePaneToFile(sess.TmuxSession, outputLog); err != nil {
		m.tmux.KillSession(sess.TmuxSession)
		m.worktrees.Remove(repo.Path, wtPath)
		return nil, fmt.Errorf("setting up output capture: %w", err)
	}

	sess.Status = StatusRunning
	m.sessions[id] = sess

	go m.monitorSession(sess)

	m.notifyStatus(sess)
	return sess, nil
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
	m.mu.Unlock()

	m.tmux.SendKeys(sess.TmuxSession, "C-c")
	time.Sleep(500 * time.Millisecond)
	m.tmux.KillSession(sess.TmuxSession)

	m.mu.Lock()
	now := time.Now()
	sess.Status = StatusStopped
	sess.CompletedAt = &now
	m.mu.Unlock()

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
		go m.pollCapturePane(ctx, tmuxName, ch)
	} else {
		outputLog := filepath.Join(m.outputDir, sessionID, "output.log")
		go m.sendFileOnce(ctx, outputLog, ch)
	}

	cancelFn := func() {
		cancel()
		close(ch)
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
	m.mu.RUnlock()

	return m.tmux.SendKeys(sess.TmuxSession, input)
}

func (m *Manager) Reconcile() error {
	names, err := m.tmux.ListSessions(tmuxPrefix)
	if err != nil {
		return fmt.Errorf("listing tmux sessions: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, name := range names {
		id := strings.TrimPrefix(name, tmuxPrefix)
		if _, exists := m.sessions[id]; exists {
			continue
		}

		sess := &Session{
			ID:          id,
			TmuxSession: name,
			Status:      StatusRunning,
			CreatedAt:   time.Now(),
		}
		m.sessions[id] = sess
		go m.monitorSession(sess)
	}

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

func (m *Manager) monitorSession(sess *Session) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.RLock()
		if !sess.IsActive() {
			m.mu.RUnlock()
			return
		}
		m.mu.RUnlock()

		if !m.tmux.HasSession(sess.TmuxSession) {
			m.mu.Lock()
			now := time.Now()
			sess.CompletedAt = &now

			exitCode := m.detectExitCode(sess.ID)
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
			m.mu.Unlock()
			m.handlePostSession(sess)
			m.notifyStatus(sess)
			return
		}

		waitingInput := m.checkWaitingInput(sess)
		m.mu.Lock()
		if sess.WaitingInput != waitingInput {
			sess.WaitingInput = waitingInput
			m.mu.Unlock()
			m.notifyStatus(sess)
		} else {
			m.mu.Unlock()
		}
	}
}

func (m *Manager) checkWaitingInput(sess *Session) bool {
	pid, err := m.tmux.PanePID(sess.TmuxSession)
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

func (m *Manager) handlePostSession(sess *Session) {
	if m.postHandler != nil {
		postSession := m.runner.GetPostSession(sess.Agent)
		m.postHandler(sess.RepoPath, sess.Worktree, postSession)
	}
}

func (m *Manager) notifyStatus(sess *Session) {
	if m.onStatus != nil {
		cp := *sess
		m.onStatus(&cp)
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
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	stripped := stripANSI(data)
	select {
	case ch <- stripped:
	case <-ctx.Done():
	}
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b\[[\?]?[0-9;]*[a-zA-Z]`)

func stripANSI(data []byte) []byte {
	return ansiRe.ReplaceAll(data, nil)
}
