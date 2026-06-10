package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/creydr/ai-mux/internal/config"
	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/poller"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/session"
	"github.com/creydr/ai-mux/internal/store"
	"github.com/creydr/ai-mux/internal/worktree"
)

type Daemon struct {
	config     *config.Config
	provider   provider.Provider
	store      store.Store
	bus        *event.Bus
	poller     *poller.Poller
	listener   protocol.Listener
	sessionMgr *session.Manager

	clients   map[string]*clientConn
	clientsMu sync.RWMutex
	nextID    int

	ctx       context.Context
	startTime time.Time
	cancel    context.CancelFunc
}

type clientConn struct {
	id           string
	conn         protocol.Conn
	eventCh      <-chan event.Event
	done         chan struct{}
	outputCancel func()
}

func New(cfg *config.Config, prov provider.Provider, st store.Store, transport protocol.Transport) (*Daemon, error) {
	bus := event.NewBus()

	repos := make([]provider.RepoRef, len(cfg.Repos))
	for i, r := range cfg.Repos {
		ref, err := provider.ParseRepoRef(r.Name)
		if err != nil {
			return nil, fmt.Errorf("parsing repo %q: %w", r.Name, err)
		}
		repos[i] = ref
	}

	p := poller.New(prov, st, bus, repos, cfg.PollInterval.Duration)

	ln, err := transport.Listen(cfg.Daemon.Socket)
	if err != nil {
		return nil, fmt.Errorf("starting listener: %w", err)
	}

	var sessMgr *session.Manager
	if len(cfg.Agents) > 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting home directory: %w", err)
		}
		sessStore := session.NewStore(filepath.Join(home, ".ai-mux"))
		sessMgr = session.NewManager(session.ManagerConfig{
			Agents: cfg.Agents,
			Repos:  cfg.Repos,
			Store:  sessStore,
		})
		sessMgr.Reconcile()
	}

	d := &Daemon{
		config:     cfg,
		provider:   prov,
		store:      st,
		bus:        bus,
		poller:     p,
		listener:   ln,
		sessionMgr: sessMgr,
		clients:    make(map[string]*clientConn),
	}

	if sessMgr != nil {
		sessMgr.SetOnStatus(func(sess *session.Session) {
			p := sessionToPayload(sess)
			d.bus.Publish(event.Event{
				Type:      event.TypeSessionStatus,
				Session:   &p,
				Timestamp: time.Now(),
			})
		})
	}

	return d, nil
}

func (d *Daemon) Start(ctx context.Context) error {
	d.ctx, d.cancel = context.WithCancel(ctx)
	d.startTime = time.Now()
	ctx = d.ctx

	go func() {
		if err := d.poller.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("poller error: %v", err)
		}
	}()

	go d.acceptLoop(ctx)

	<-ctx.Done()
	return d.cleanup()
}

func (d *Daemon) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
}

func (d *Daemon) cleanup() error {
	d.clientsMu.Lock()
	for _, c := range d.clients {
		c.conn.Close()
	}
	d.clients = make(map[string]*clientConn)
	d.clientsMu.Unlock()

	d.bus.Close()
	d.listener.Close()
	return d.store.Close()
}

func (d *Daemon) acceptLoop(ctx context.Context) {
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("accept error: %v", err)
			continue
		}

		d.clientsMu.Lock()
		id := fmt.Sprintf("client-%d", d.nextID)
		d.nextID++
		cc := &clientConn{
			id:   id,
			conn: conn,
			done: make(chan struct{}),
		}
		d.clients[id] = cc
		d.clientsMu.Unlock()

		go d.handleClient(ctx, cc)
	}
}

func (d *Daemon) handleClient(ctx context.Context, cc *clientConn) {
	defer func() {
		close(cc.done)
		if cc.outputCancel != nil {
			cc.outputCancel()
		}
		d.clientsMu.Lock()
		delete(d.clients, cc.id)
		d.clientsMu.Unlock()
		if cc.eventCh != nil {
			d.bus.Unsubscribe(cc.eventCh)
		}
		cc.conn.Close()
	}()

	go func() {
		select {
		case <-ctx.Done():
		case <-cc.done:
		}
		cc.conn.Close()
	}()

	for {
		msg, err := cc.conn.Receive()
		if err != nil {
			return
		}

		d.handleMessage(cc, msg)
	}
}

func (d *Daemon) handleMessage(cc *clientConn, msg protocol.Message) {
	switch msg.Type {
	case protocol.MsgSubscribe:
		d.handleSubscribe(cc, msg)
	case protocol.MsgListIssues:
		d.handleListItems(cc, msg, provider.ItemTypeIssue)
	case protocol.MsgListPRs:
		d.handleListItems(cc, msg, provider.ItemTypePR)
	case protocol.MsgGetItem:
		d.handleGetItem(cc, msg)
	case protocol.MsgMarkRead:
		d.handleMarkRead(cc, msg)
	case protocol.MsgGetStatus:
		d.handleGetStatus(cc, msg)
	case protocol.MsgSessionSpawn:
		d.handleSessionSpawn(cc, msg)
	case protocol.MsgSessionList:
		d.handleSessionList(cc, msg)
	case protocol.MsgSessionStop:
		d.handleSessionStop(cc, msg)
	case protocol.MsgSessionAttach:
		d.handleSessionAttach(cc, msg)
	case protocol.MsgSessionDetach:
		d.handleSessionDetach(cc, msg)
	case protocol.MsgSessionInput:
		d.handleSessionInput(cc, msg)
	case protocol.MsgSessionRename:
		d.handleSessionRename(cc, msg)
	default:
		resp, _ := protocol.NewError(msg.ID, fmt.Sprintf("unknown message type: %s", msg.Type))
		cc.conn.Send(resp)
	}
}

func (d *Daemon) handleSubscribe(cc *clientConn, msg protocol.Message) {
	ch := d.bus.Subscribe()
	cc.eventCh = ch

	go func() {
		for ev := range ch {
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			evMsg := protocol.Message{
				Type:    protocol.MsgEvent,
				Payload: data,
			}
			if err := cc.conn.Send(evMsg); err != nil {
				return
			}
		}
	}()

	resp, _ := protocol.NewResponse(msg.ID, map[string]string{"status": "subscribed"})
	cc.conn.Send(resp)
}

func (d *Daemon) handleListItems(cc *clientConn, msg protocol.Message, itemType provider.ItemType) {
	payload, err := protocol.ParsePayload[protocol.ListPayload](msg)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	repos := d.config.Repos
	if payload.Repo != "" {
		repos = []config.RepoConfig{{Name: payload.Repo}}
	}

	results := make([][]provider.Item, len(repos))
	var wg sync.WaitGroup

	for i, r := range repos {
		ref, err := provider.ParseRepoRef(r.Name)
		if err != nil {
			continue
		}
		wg.Add(1)
		go func(idx int, ref provider.RepoRef) {
			defer wg.Done()
			var items []provider.Item
			var fetchErr error
			opts := provider.ListOptions{State: "open", Limit: payload.Limit}
			if itemType == provider.ItemTypeIssue {
				items, fetchErr = d.provider.ListIssues(d.ctx, ref, opts)
			} else {
				items, fetchErr = d.provider.ListPRs(d.ctx, ref, opts)
			}
			if fetchErr != nil {
				log.Printf("error listing %s for %s: %v", itemType, ref, fetchErr)
				return
			}
			results[idx] = items
		}(i, ref)
	}
	wg.Wait()

	var allItems []provider.Item
	for _, items := range results {
		allItems = append(allItems, items...)
	}

	itemsJSON, _ := json.Marshal(allItems)
	resp, _ := protocol.NewResponse(msg.ID, protocol.ItemsPayload{Items: itemsJSON})
	cc.conn.Send(resp)
}

func (d *Daemon) handleGetItem(cc *clientConn, msg protocol.Message) {
	payload, err := protocol.ParsePayload[protocol.GetItemPayload](msg)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	ref, err := provider.ParseRepoRef(payload.Repo)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	item, err := d.provider.GetItem(d.ctx, ref, provider.ItemType(payload.Type), payload.Number)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	resp, _ := protocol.NewResponse(msg.ID, item)
	cc.conn.Send(resp)
}

func (d *Daemon) handleMarkRead(cc *clientConn, msg protocol.Message) {
	payload, err := protocol.ParsePayload[protocol.MarkReadPayload](msg)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	if err := d.store.MarkRead(payload.ItemID); err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	resp, _ := protocol.NewResponse(msg.ID, map[string]string{"status": "ok"})
	cc.conn.Send(resp)
}

func (d *Daemon) handleGetStatus(cc *clientConn, msg protocol.Message) {
	d.clientsMu.RLock()
	clientCount := len(d.clients)
	d.clientsMu.RUnlock()

	repos := make([]string, len(d.config.Repos))
	for i, r := range d.config.Repos {
		repos[i] = r.Name
	}

	resp, _ := protocol.NewResponse(msg.ID, protocol.StatusPayload{
		Running: true,
		Repos:   repos,
		Clients: clientCount,
		Uptime:  time.Since(d.startTime).Truncate(time.Second).String(),
	})
	cc.conn.Send(resp)
}

func (d *Daemon) handleSessionSpawn(cc *clientConn, msg protocol.Message) {
	if d.sessionMgr == nil {
		resp, _ := protocol.NewError(msg.ID, "no agents configured")
		cc.conn.Send(resp)
		return
	}

	payload, err := protocol.ParsePayload[protocol.SessionSpawnPayload](msg)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	wtAction := session.WorktreeAction(payload.WorktreeAction)
	sess, err := d.sessionMgr.Spawn(payload.Repo, payload.Number, payload.ItemType, payload.Agent, wtAction)
	if err != nil {
		if errors.Is(err, worktree.ErrWorktreeExists) {
			resp, err := protocol.NewRequest(protocol.MsgWorktreeExists, msg.ID, payload)
			if err != nil {
				log.Printf("error creating worktree-exists response: %v", err)
				return
			}
			_ = cc.conn.Send(resp)
			return
		}
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	resp, _ := protocol.NewResponse(msg.ID, sessionToPayload(sess))
	cc.conn.Send(resp)
}

func (d *Daemon) handleSessionList(cc *clientConn, msg protocol.Message) {
	if d.sessionMgr == nil {
		resp, _ := protocol.NewResponse(msg.ID, protocol.SessionListPayload{})
		cc.conn.Send(resp)
		return
	}

	sessions := d.sessionMgr.List()
	payloads := make([]protocol.SessionPayload, len(sessions))
	for i, s := range sessions {
		payloads[i] = sessionToPayload(s)
	}

	resp, _ := protocol.NewResponse(msg.ID, protocol.SessionListPayload{Sessions: payloads})
	cc.conn.Send(resp)
}

func (d *Daemon) handleSessionStop(cc *clientConn, msg protocol.Message) {
	if d.sessionMgr == nil {
		resp, _ := protocol.NewError(msg.ID, "no agents configured")
		cc.conn.Send(resp)
		return
	}

	payload, err := protocol.ParsePayload[protocol.SessionIDPayload](msg)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	if err := d.sessionMgr.Stop(payload.SessionID); err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	resp, _ := protocol.NewResponse(msg.ID, map[string]string{"status": "stopped"})
	cc.conn.Send(resp)
}

func (d *Daemon) handleSessionAttach(cc *clientConn, msg protocol.Message) {
	if d.sessionMgr == nil {
		resp, _ := protocol.NewError(msg.ID, "no agents configured")
		cc.conn.Send(resp)
		return
	}

	payload, err := protocol.ParsePayload[protocol.SessionIDPayload](msg)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	ch, cancel, err := d.sessionMgr.AttachOutput(payload.SessionID)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	if cc.outputCancel != nil {
		cc.outputCancel()
	}
	cc.outputCancel = cancel

	go func() {
		for data := range ch {
			outMsg, err := protocol.NewRequest(protocol.MsgSessionOutput, "", protocol.SessionOutputPayload{
				SessionID: payload.SessionID,
				Data:      string(data),
			})
			if err != nil {
				continue
			}
			if err := cc.conn.Send(outMsg); err != nil {
				cancel()
				return
			}
		}
	}()

	resp, _ := protocol.NewResponse(msg.ID, map[string]string{"status": "attached"})
	cc.conn.Send(resp)
}

func (d *Daemon) handleSessionDetach(cc *clientConn, msg protocol.Message) {
	if cc.outputCancel != nil {
		cc.outputCancel()
		cc.outputCancel = nil
	}

	resp, _ := protocol.NewResponse(msg.ID, map[string]string{"status": "detached"})
	cc.conn.Send(resp)
}

func (d *Daemon) handleSessionInput(cc *clientConn, msg protocol.Message) {
	if d.sessionMgr == nil {
		resp, _ := protocol.NewError(msg.ID, "no agents configured")
		cc.conn.Send(resp)
		return
	}

	payload, err := protocol.ParsePayload[protocol.SessionInputPayload](msg)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	if err := d.sessionMgr.SendInput(payload.SessionID, payload.Input); err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	resp, _ := protocol.NewResponse(msg.ID, map[string]string{"status": "sent"})
	cc.conn.Send(resp)
}

func (d *Daemon) handleSessionRename(cc *clientConn, msg protocol.Message) {
	if d.sessionMgr == nil {
		resp, _ := protocol.NewError(msg.ID, "no agents configured")
		cc.conn.Send(resp)
		return
	}

	payload, err := protocol.ParsePayload[protocol.SessionRenamePayload](msg)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	if err := d.sessionMgr.Rename(payload.SessionID, payload.Name); err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.conn.Send(resp)
		return
	}

	resp, _ := protocol.NewResponse(msg.ID, map[string]string{"status": "renamed"})
	cc.conn.Send(resp)
}

func sessionToPayload(s *session.Session) protocol.SessionPayload {
	return protocol.SessionPayload{
		ID:           s.ID,
		Name:         s.Name,
		Repo:         s.ItemRepo,
		Number:       s.ItemNumber,
		ItemType:     s.ItemType,
		Agent:        s.Agent,
		Status:       string(s.Status),
		WaitingInput: s.WaitingInput,
		Worktree:     s.Worktree,
		CreatedAt:    s.CreatedAt.Format(time.RFC3339),
	}
}
