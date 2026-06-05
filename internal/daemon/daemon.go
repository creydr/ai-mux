package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/creydr/ai-mux/internal/config"
	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/poller"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/store"
)

type Daemon struct {
	config   *config.Config
	provider provider.Provider
	store    store.Store
	bus      *event.Bus
	poller   *poller.Poller
	listener protocol.Listener

	clients   map[string]*clientConn
	clientsMu sync.RWMutex
	nextID    int

	startTime time.Time
	cancel    context.CancelFunc
}

type clientConn struct {
	id      string
	conn    protocol.Conn
	eventCh <-chan event.Event
	done    chan struct{}
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

	ln, err := transport.Listen(cfg.ACP.Socket)
	if err != nil {
		return nil, fmt.Errorf("starting listener: %w", err)
	}

	return &Daemon{
		config:   cfg,
		provider: prov,
		store:    st,
		bus:      bus,
		poller:   p,
		listener: ln,
		clients:  make(map[string]*clientConn),
	}, nil
}

func (d *Daemon) Start(ctx context.Context) error {
	ctx, d.cancel = context.WithCancel(ctx)
	d.startTime = time.Now()

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
		close(c.done)
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
		d.clientsMu.Lock()
		delete(d.clients, cc.id)
		d.clientsMu.Unlock()
		if cc.eventCh != nil {
			d.bus.Unsubscribe(cc.eventCh)
		}
		cc.conn.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-cc.done:
			return
		default:
		}

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

	type result struct {
		items []provider.Item
	}

	results := make([]result, len(repos))
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
				items, fetchErr = d.provider.ListIssues(context.Background(), ref, opts)
			} else {
				items, fetchErr = d.provider.ListPRs(context.Background(), ref, opts)
			}
			if fetchErr != nil {
				log.Printf("error listing %s for %s: %v", itemType, ref, fetchErr)
				return
			}
			results[idx] = result{items: items}
		}(i, ref)
	}
	wg.Wait()

	var allItems []provider.Item
	for _, r := range results {
		allItems = append(allItems, r.items...)
	}

	resp, _ := protocol.NewResponse(msg.ID, protocol.ItemsPayload{Items: allItems})
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

	item, err := d.provider.GetItem(context.Background(), ref, provider.ItemType(payload.Type), payload.Number)
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
