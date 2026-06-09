//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/creydr/ai-mux/internal/config"
	"github.com/creydr/ai-mux/internal/daemon"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/protocol/jsonlines"
	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/provider/mock"
	"github.com/creydr/ai-mux/internal/store"
	"github.com/creydr/ai-mux/internal/store/jsonfile"
)

func TestIntegration_FullDaemonFlow(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")
	statePath := filepath.Join(tmpDir, "state.json")

	cfg := &config.Config{
		Repos: []config.RepoConfig{
			{Name: "test/repo", Path: "/tmp/test-repo"},
		},
		PollInterval: config.Duration{Duration: time.Hour},
		Daemon:       config.DaemonConfig{Socket: socketPath},
	}

	prov := mock.New()
	prov.AddIssues(provider.RepoRef{Owner: "test", Repo: "repo"},
		provider.Item{ID: "test/repo/issues/1", Number: 1, Title: "Integration bug", Type: provider.ItemTypeIssue, UpdatedAt: time.Now()},
		provider.Item{ID: "test/repo/issues/2", Number: 2, Title: "Feature request", Type: provider.ItemTypeIssue, UpdatedAt: time.Now()},
	)
	prov.AddPRs(provider.RepoRef{Owner: "test", Repo: "repo"},
		provider.Item{ID: "test/repo/prs/10", Number: 10, Title: "Fix tests", Type: provider.ItemTypePR, UpdatedAt: time.Now()},
	)

	st, err := jsonfile.New(statePath)
	if err != nil {
		t.Fatal(err)
	}

	transport := jsonlines.NewTransport()
	d, err := daemon.New(cfg, prov, st, transport)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go d.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	conn, err := transport.Dial(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// List issues
	msg, _ := protocol.NewRequest(protocol.MsgListIssues, "1", protocol.ListPayload{})
	conn.Send(msg)
	resp, err := conn.Receive()
	if err != nil {
		t.Fatal(err)
	}
	var issues protocol.ItemsPayload
	json.Unmarshal(resp.Payload, &issues)
	if len(issues.Items) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues.Items))
	}

	// List PRs
	msg, _ = protocol.NewRequest(protocol.MsgListPRs, "2", protocol.ListPayload{})
	conn.Send(msg)
	resp, _ = conn.Receive()
	var prs protocol.ItemsPayload
	json.Unmarshal(resp.Payload, &prs)
	if len(prs.Items) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs.Items))
	}

	// Get status
	msg, _ = protocol.NewRequest(protocol.MsgGetStatus, "3", nil)
	conn.Send(msg)
	resp, _ = conn.Receive()
	var status protocol.StatusPayload
	json.Unmarshal(resp.Payload, &status)
	if !status.Running {
		t.Error("expected running=true")
	}

	// Mark read
	st.SetItemState(store.ItemState{ItemID: "test/repo/issues/1", Read: false, LastSeenAt: time.Now()})
	msg, _ = protocol.NewRequest(protocol.MsgMarkRead, "4", protocol.MarkReadPayload{ItemID: "test/repo/issues/1"})
	conn.Send(msg)
	resp, _ = conn.Receive()
	if resp.Type != protocol.MsgResponse {
		t.Fatalf("expected response, got %s", resp.Type)
	}

	state, _ := st.GetItemState("test/repo/issues/1")
	if !state.Read {
		t.Error("item should be marked as read")
	}

	// Second client
	conn2, err := transport.Dial(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close()

	msg, _ = protocol.NewRequest(protocol.MsgGetStatus, "5", nil)
	conn2.Send(msg)
	resp, _ = conn2.Receive()
	json.Unmarshal(resp.Payload, &status)
	if status.Clients < 2 {
		t.Errorf("expected at least 2 clients, got %d", status.Clients)
	}

	cancel()
}

func TestIntegration_RestartPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	st1, _ := jsonfile.New(statePath)
	st1.SetItemState(store.ItemState{ItemID: "test/item", Read: true, LastSeenAt: time.Now()})
	st1.Close()

	st2, _ := jsonfile.New(statePath)
	defer st2.Close()

	state, err := st2.GetItemState("test/item")
	if err != nil {
		t.Fatal(err)
	}
	if state == nil {
		t.Fatal("state should be persisted")
	}
	if !state.Read {
		t.Error("read status should be persisted")
	}
}
