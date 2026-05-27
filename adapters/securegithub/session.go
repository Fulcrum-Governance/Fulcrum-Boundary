package securegithub

import "sync"

type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*SessionState
}

type SessionState struct {
	ID           string
	BoundOwner   string
	BoundRepo    string
	Tainted      bool
	TaintSources []string
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: map[string]*SessionState{}}
}

func (s *SessionStore) Get(id string) *SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		id = DefaultSessionID
	}
	state := s.sessions[id]
	if state == nil {
		state = &SessionState{ID: id}
		s.sessions[id] = state
	}
	return cloneSessionState(state)
}

func (s *SessionStore) BindRepo(id, owner, repo string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.ensureLocked(id)
	if state.BoundOwner == "" && state.BoundRepo == "" {
		state.BoundOwner = owner
		state.BoundRepo = repo
		return true
	}
	return state.BoundOwner == owner && state.BoundRepo == repo
}

func (s *SessionStore) MarkTainted(id, source string) {
	if source == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.ensureLocked(id)
	state.Tainted = true
	for _, existing := range state.TaintSources {
		if existing == source {
			return
		}
	}
	state.TaintSources = append(state.TaintSources, source)
}

func (s *SessionStore) ensureLocked(id string) *SessionState {
	if id == "" {
		id = DefaultSessionID
	}
	state := s.sessions[id]
	if state == nil {
		state = &SessionState{ID: id}
		s.sessions[id] = state
	}
	return state
}

func cloneSessionState(in *SessionState) *SessionState {
	if in == nil {
		return &SessionState{ID: DefaultSessionID}
	}
	return &SessionState{
		ID:           in.ID,
		BoundOwner:   in.BoundOwner,
		BoundRepo:    in.BoundRepo,
		Tainted:      in.Tainted,
		TaintSources: append([]string{}, in.TaintSources...),
	}
}
