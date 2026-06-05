package daemon

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/creydr/ai-mux/internal/config"
	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/protocol/jsonlines"
	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/provider/mock"
	"github.com/creydr/ai-mux/internal/store"
	"github.com/creydr/ai-mux/internal/store/jsonfile"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Repos:        []config.RepoConfig{{Name: "owner/repo", Path: "/tmp/repo"}},
		PollInterval: config.Duration{Duration: time.Hour},
		ACP:          config.ACPConfig{Socket: filepath.Join(t.TempDir(), "test.sock")},
	}
}

func startDaemon(t *testing.T, cfg *config.Config, prov provider.Provider) (*Daemon, context.CancelFunc) {
	t.Helper()
	st, err := jsonfile.New(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	transport := jsonlines.NewTransport()
	d, err := New(cfg, prov, st, transport)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	go func() {
		close(started)
		d.Start(ctx)
	}()
	<-started
	time.Sleep(50 * time.Millisecond)

	t.Cleanup(func() {
		cancel()
		time.Sleep(50 * time.Millisecond)
	})

	return d, cancel
}

func dial(t *testing.T, socket string) protocol.Conn {
	t.Helper()
	transport := jsonlines.NewTransport()
	conn, err := transport.Dial(socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func sendRequest(t *testing.T, conn protocol.Conn, msgType protocol.MessageType, id string, payload any) {
	t.Helper()
	msg, err := protocol.NewRequest(msgType, id, payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Send(msg); err != nil {
		t.Fatal(err)
	}
}

func receiveWithTimeout(t *testing.T, conn protocol.Conn, timeout time.Duration) protocol.Message {
	t.Helper()
	type result struct {
		msg protocol.Message
		err error
	}
	ch := make(chan result, 1)
	go func() {
		msg, err := conn.Receive()
		ch <- result{msg, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatal(r.err)
		}
		return r.msg
	case <-time.After(timeout):
		t.Fatal("timed out waiting for response")
		return protocol.Message{}
	}
}

func TestDaemon_StartAndStop(t *testing.T) {
	cfg := testConfig(t)
	prov := mock.New()

	_, cancel := startDaemon(t, cfg, prov)

	conn := dial(t, cfg.ACP.Socket)
	sendRequest(t, conn, protocol.MsgGetStatus, "1", nil)
	resp := receiveWithTimeout(t, conn, 2*time.Second)

	if resp.Type != protocol.MsgResponse {
		t.Fatalf("expected response, got %s", resp.Type)
	}

	var status protocol.StatusPayload
	json.Unmarshal(resp.Payload, &status)
	if !status.Running {
		t.Error("expected running=true")
	}
	if len(status.Repos) != 1 || status.Repos[0] != "owner/repo" {
		t.Errorf("unexpected repos: %v", status.Repos)
	}

	cancel()
}

func TestDaemon_ListIssues(t *testing.T) {
	cfg := testConfig(t)
	prov := mock.New()
	prov.AddIssues(provider.RepoRef{Owner: "owner", Repo: "repo"},
		provider.Item{ID: "owner/repo/issues/1", Number: 1, Title: "Bug", Type: provider.ItemTypeIssue, UpdatedAt: time.Now()},
	)

	startDaemon(t, cfg, prov)

	conn := dial(t, cfg.ACP.Socket)
	sendRequest(t, conn, protocol.MsgListIssues, "1", protocol.ListPayload{})
	resp := receiveWithTimeout(t, conn, 2*time.Second)

	var items protocol.ItemsPayload
	json.Unmarshal(resp.Payload, &items)

	if len(items.Items) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(items.Items))
	}
	if items.Items[0].Title != "Bug" {
		t.Errorf("expected title 'Bug', got %q", items.Items[0].Title)
	}
}

func TestDaemon_ListPRs(t *testing.T) {
	cfg := testConfig(t)
	prov := mock.New()
	prov.AddPRs(provider.RepoRef{Owner: "owner", Repo: "repo"},
		provider.Item{ID: "owner/repo/prs/10", Number: 10, Title: "Feature", Type: provider.ItemTypePR, UpdatedAt: time.Now()},
	)

	startDaemon(t, cfg, prov)

	conn := dial(t, cfg.ACP.Socket)
	sendRequest(t, conn, protocol.MsgListPRs, "1", protocol.ListPayload{})
	resp := receiveWithTimeout(t, conn, 2*time.Second)

	var items protocol.ItemsPayload
	json.Unmarshal(resp.Payload, &items)

	if len(items.Items) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(items.Items))
	}
	if items.Items[0].Title != "Feature" {
		t.Errorf("expected title 'Feature', got %q", items.Items[0].Title)
	}
}

func TestDaemon_ListIssues_FilteredByRepo(t *testing.T) {
	cfg := &config.Config{
		Repos: []config.RepoConfig{
			{Name: "owner/repo1", Path: "/tmp/repo1"},
			{Name: "owner/repo2", Path: "/tmp/repo2"},
		},
		PollInterval: config.Duration{Duration: time.Hour},
		ACP:          config.ACPConfig{Socket: filepath.Join(t.TempDir(), "test.sock")},
	}
	prov := mock.New()
	prov.AddIssues(provider.RepoRef{Owner: "owner", Repo: "repo1"},
		provider.Item{ID: "owner/repo1/issues/1", Number: 1, Title: "R1", UpdatedAt: time.Now()},
	)
	prov.AddIssues(provider.RepoRef{Owner: "owner", Repo: "repo2"},
		provider.Item{ID: "owner/repo2/issues/1", Number: 1, Title: "R2", UpdatedAt: time.Now()},
	)

	startDaemon(t, cfg, prov)

	conn := dial(t, cfg.ACP.Socket)
	sendRequest(t, conn, protocol.MsgListIssues, "1", protocol.ListPayload{Repo: "owner/repo1"})
	resp := receiveWithTimeout(t, conn, 2*time.Second)

	var items protocol.ItemsPayload
	json.Unmarshal(resp.Payload, &items)

	if len(items.Items) != 1 {
		t.Fatalf("expected 1 issue from repo1, got %d", len(items.Items))
	}
	if items.Items[0].Title != "R1" {
		t.Errorf("expected title 'R1', got %q", items.Items[0].Title)
	}
}

func TestDaemon_GetItem(t *testing.T) {
	cfg := testConfig(t)
	prov := mock.New()
	prov.AddIssues(provider.RepoRef{Owner: "owner", Repo: "repo"},
		provider.Item{ID: "owner/repo/issues/5", Number: 5, Title: "Find me", Type: provider.ItemTypeIssue, UpdatedAt: time.Now()},
	)

	startDaemon(t, cfg, prov)

	conn := dial(t, cfg.ACP.Socket)
	sendRequest(t, conn, protocol.MsgGetItem, "1", protocol.GetItemPayload{Repo: "owner/repo", Type: "issue", Number: 5})
	resp := receiveWithTimeout(t, conn, 2*time.Second)

	var item provider.Item
	json.Unmarshal(resp.Payload, &item)

	if item.Title != "Find me" {
		t.Errorf("expected title 'Find me', got %q", item.Title)
	}
}

func TestDaemon_MarkRead(t *testing.T) {
	cfg := testConfig(t)
	prov := mock.New()

	d, _ := startDaemon(t, cfg, prov)

	d.store.SetItemState(store.ItemState{ItemID: "test-item", Read: false, LastSeenAt: time.Now()})

	conn := dial(t, cfg.ACP.Socket)
	sendRequest(t, conn, protocol.MsgMarkRead, "1", protocol.MarkReadPayload{ItemID: "test-item"})
	resp := receiveWithTimeout(t, conn, 2*time.Second)

	if resp.Type != protocol.MsgResponse {
		t.Fatalf("expected response, got %s", resp.Type)
	}

	state, _ := d.store.GetItemState("test-item")
	if state == nil || !state.Read {
		t.Error("expected item to be marked as read")
	}
}

func TestDaemon_Subscribe_ReceivesEvents(t *testing.T) {
	cfg := testConfig(t)
	prov := mock.New()

	d, _ := startDaemon(t, cfg, prov)

	conn := dial(t, cfg.ACP.Socket)
	sendRequest(t, conn, protocol.MsgSubscribe, "1", nil)

	subResp := receiveWithTimeout(t, conn, 2*time.Second)
	if subResp.Type != protocol.MsgResponse {
		t.Fatalf("expected subscribe response, got %s", subResp.Type)
	}

	d.bus.Publish(event.Event{
		Type:      event.TypeNewIssue,
		Item:      &provider.Item{ID: "test", Title: "New one"},
		Timestamp: time.Now(),
	})

	evMsg := receiveWithTimeout(t, conn, 2*time.Second)
	if evMsg.Type != protocol.MsgEvent {
		t.Fatalf("expected event, got %s", evMsg.Type)
	}
}

func TestDaemon_MultipleClients(t *testing.T) {
	cfg := testConfig(t)
	prov := mock.New()
	prov.AddIssues(provider.RepoRef{Owner: "owner", Repo: "repo"},
		provider.Item{ID: "owner/repo/issues/1", Number: 1, Title: "Shared", UpdatedAt: time.Now()},
	)

	startDaemon(t, cfg, prov)

	conn1 := dial(t, cfg.ACP.Socket)
	conn2 := dial(t, cfg.ACP.Socket)

	sendRequest(t, conn1, protocol.MsgListIssues, "1", protocol.ListPayload{})
	sendRequest(t, conn2, protocol.MsgListIssues, "2", protocol.ListPayload{})

	resp1 := receiveWithTimeout(t, conn1, 2*time.Second)
	resp2 := receiveWithTimeout(t, conn2, 2*time.Second)

	var items1, items2 protocol.ItemsPayload
	json.Unmarshal(resp1.Payload, &items1)
	json.Unmarshal(resp2.Payload, &items2)

	if len(items1.Items) != 1 || len(items2.Items) != 1 {
		t.Errorf("both clients should get 1 item, got %d and %d", len(items1.Items), len(items2.Items))
	}
}

func TestDaemon_UnknownMessageType(t *testing.T) {
	cfg := testConfig(t)
	prov := mock.New()

	startDaemon(t, cfg, prov)

	conn := dial(t, cfg.ACP.Socket)
	sendRequest(t, conn, "bogus_type", "1", nil)
	resp := receiveWithTimeout(t, conn, 2*time.Second)

	if resp.Type != protocol.MsgError {
		t.Fatalf("expected error response, got %s", resp.Type)
	}
}
