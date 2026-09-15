package audit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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

func (m *mockObserver) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

func TestService_NoObservers(t *testing.T) {
	svc := NewService()
	defer svc.Close()

	assert.False(t, svc.HasObservers())

	// Не должно паниковать
	svc.NotifyAll(context.Background(), Event{Action: ActionShorten, URL: "http://example.com"})
}

func TestService_NotifyAll(t *testing.T) {
	svc := NewService()
	defer svc.Close()

	obs1 := &mockObserver{}
	obs2 := &mockObserver{}
	svc.Subscribe(obs1)
	svc.Subscribe(obs2)

	svc.NotifyAll(context.Background(), Event{Action: "test"})

	require.Eventually(t, func() bool {
		return obs1.Calls() == 1 && obs2.Calls() == 1
	}, time.Second, 10*time.Millisecond)

	require.Equal(t, 1, len(obs1.Events()))
	require.Equal(t, "test", string(obs1.Events()[0].Action))
}

func TestService_ContinuesOnError(t *testing.T) {
	svc := NewService()
	defer svc.Close()

	failing := &mockObserver{err: errors.New("boom")}
	ok := &mockObserver{}
	svc.Subscribe(failing)
	svc.Subscribe(ok)

	svc.NotifyAll(context.Background(), Event{Action: ActionShorten, URL: "http://example.com"})

	// Ждём, пока оба observer'а обработают событие.
	// Ошибка failing не должна помешать ok получить событие.
	require.Eventually(t, func() bool {
		return failing.Calls() == 1 && ok.Calls() == 1
	}, time.Second, 10*time.Millisecond)

	require.Equal(t, 1, len(ok.Events()))
	require.Equal(t, ActionShorten, ok.Events()[0].Action)
}
