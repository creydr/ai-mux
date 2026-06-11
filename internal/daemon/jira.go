package daemon

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/jira"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
)

type jiraState struct {
	mu     sync.RWMutex
	client *jira.Client
	items  []provider.JiraItem
}

func (d *Daemon) pollJira(ctx context.Context) {
	if d.jira == nil {
		return
	}

	if err := d.fetchJiraItems(ctx); err != nil {
		log.Printf("initial jira poll error: %v", err)
	}

	ticker := time.NewTicker(d.config.PollInterval.Duration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := d.fetchJiraItems(ctx); err != nil {
				log.Printf("jira poll error: %v", err)
			}
		}
	}
}

func (d *Daemon) fetchJiraItems(ctx context.Context) error {
	items, err := d.jira.client.Search(ctx, d.config.Jira.JQL, d.config.Jira.OrderBy, d.config.Jira.MaxResults)
	if err != nil {
		return err
	}

	d.jira.mu.Lock()
	oldKeys := make(map[string]*provider.JiraItem, len(d.jira.items))
	for i := range d.jira.items {
		oldKeys[d.jira.items[i].Key] = &d.jira.items[i]
	}
	d.jira.items = items
	d.jira.mu.Unlock()

	for i := range items {
		item := items[i]
		old, exists := oldKeys[item.Key]
		if !exists {
			d.bus.Publish(event.Event{
				Type:      event.TypeNewJiraItem,
				JiraItem:  &item,
				Timestamp: time.Now(),
			})
		} else if old.Status != item.Status || old.Summary != item.Summary || old.Assignee != item.Assignee || old.Priority != item.Priority {
			d.bus.Publish(event.Event{
				Type:      event.TypeJiraItemUpdated,
				JiraItem:  &item,
				Timestamp: time.Now(),
			})
		}
	}

	return nil
}

func (d *Daemon) handleListJiraItems(cc *clientConn, msg protocol.Message) {
	if d.jira == nil {
		resp, _ := protocol.NewError(msg.ID, "jira not configured")
		cc.send(resp)
		return
	}

	payload, err := protocol.ParsePayload[protocol.JiraListPayload](msg)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.send(resp)
		return
	}

	d.jira.mu.RLock()
	allItems := d.jira.items
	d.jira.mu.RUnlock()

	total := len(allItems)
	offset := payload.Offset
	if offset > total {
		offset = total
	}
	end := total
	if payload.Limit > 0 && offset+payload.Limit < end {
		end = offset + payload.Limit
	}
	page := allItems[offset:end]

	itemsJSON, _ := json.Marshal(page)
	resp, _ := protocol.NewResponse(msg.ID, protocol.JiraItemsPayload{
		Items: itemsJSON,
		Total: total,
	})
	cc.send(resp)
}

func (d *Daemon) handleGetJiraItem(cc *clientConn, msg protocol.Message) {
	if d.jira == nil {
		resp, _ := protocol.NewError(msg.ID, "jira not configured")
		cc.send(resp)
		return
	}

	payload, err := protocol.ParsePayload[protocol.JiraKeyPayload](msg)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.send(resp)
		return
	}

	item, err := d.jira.client.GetItem(d.ctx, payload.Key)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.send(resp)
		return
	}

	resp, _ := protocol.NewResponse(msg.ID, item)
	cc.send(resp)
}

func (d *Daemon) handleGetJiraComments(cc *clientConn, msg protocol.Message) {
	if d.jira == nil {
		resp, _ := protocol.NewError(msg.ID, "jira not configured")
		cc.send(resp)
		return
	}

	payload, err := protocol.ParsePayload[protocol.JiraKeyPayload](msg)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.send(resp)
		return
	}

	comments, err := d.jira.client.GetComments(d.ctx, payload.Key)
	if err != nil {
		resp, _ := protocol.NewError(msg.ID, err.Error())
		cc.send(resp)
		return
	}

	commentsJSON, _ := json.Marshal(comments)
	resp, _ := protocol.NewResponse(msg.ID, protocol.JiraCommentsPayload{
		Comments: commentsJSON,
	})
	cc.send(resp)
}
