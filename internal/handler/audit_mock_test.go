package handler

import (
	"context"
	"sync"

	"github.com/b602op/shortener/internal/audit"
)

type mockAudit struct {
	mu     sync.Mutex
	events []audit.Event
}

func newMockAudit() *mockAudit {
	return &mockAudit{}
}

func (m *mockAudit) NotifyAll(_ context.Context, e audit.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
}

func (m *mockAudit) HasObservers() bool { return true }

func (m *mockAudit) Events() []audit.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]audit.Event, len(m.events))
	copy(out, m.events)
	return out
}
