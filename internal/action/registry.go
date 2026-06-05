package action

import "sync"

type Registry struct {
	mu      sync.RWMutex
	actions map[ActionType]Action
}

func NewRegistry() *Registry {
	return &Registry{
		actions: make(map[ActionType]Action),
	}
}

func (r *Registry) Register(a Action) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.actions[a.Type()] = a
}

func (r *Registry) Get(actionType ActionType) (Action, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.actions[actionType]
	return a, ok
}

func (r *Registry) List() []Action {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Action, 0, len(r.actions))
	for _, a := range r.actions {
		result = append(result, a)
	}
	return result
}
