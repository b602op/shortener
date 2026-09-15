package audit

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockObserver struct {
	mu     sync.Mutex
	events []Event
	err    error
}

func (m *mockObserver) Notify(_ context.Context, e Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return m.err
}

func (m *mockObserver) Events() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Event, len(m.events))
	copy(out, m.events)
	return out
}

func TestService_NoObservers(t *testing.T) {
	svc := NewService()
	assert.False(t, svc.HasObservers())

	// Не должно паниковать
	svc.NotifyAll(context.Background(), Event{Action: ActionShorten, URL: "http://example.com"})
}

func TestService_NotifyAll(t *testing.T) {
	svc := NewService()
	obs1 := &mockObserver{}
	obs2 := &mockObserver{}
	svc.Subscribe(obs1)
	svc.Subscribe(obs2)

	assert.True(t, svc.HasObservers())

	event := Event{TS: 123, Action: ActionShorten, URL: "http://example.com"}
	svc.NotifyAll(context.Background(), event)

	require.Len(t, obs1.Events(), 1)
	require.Len(t, obs2.Events(), 1)
	assert.Equal(t, event, obs1.Events()[0])
	assert.Equal(t, event, obs2.Events()[0])
}

func TestService_ContinuesOnError(t *testing.T) {
	svc := NewService()
	failing := &mockObserver{err: errors.New("boom")}
	ok := &mockObserver{}
	svc.Subscribe(failing)
	svc.Subscribe(ok)

	svc.NotifyAll(context.Background(), Event{Action: ActionShorten, URL: "http://example.com"})

	// Второй наблюдатель всё равно получил событие
	require.Len(t, ok.Events(), 1)
}
