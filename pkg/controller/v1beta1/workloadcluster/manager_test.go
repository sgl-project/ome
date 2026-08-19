package workloadcluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

// stubClient is a minimal SelectivelyCachingClient sentinel for identity checks.
type stubClient struct {
	SelectivelyCachingClient
	id string
}

func newTestManager() (*Manager, *[]string) {
	var canceled []string
	m := NewManager(runtime.NewScheme())
	m.newClient = func(_ context.Context, raw []byte, _ *runtime.Scheme) (SelectivelyCachingClient, context.CancelFunc, error) {
		id := string(raw)
		return &stubClient{id: id}, func() { canceled = append(canceled, id) }, nil
	}
	return m, &canceled
}

func TestManager_ConnectThenClientFor(t *testing.T) {
	m, _ := newTestManager()
	require.NoError(t, m.Connect(context.Background(), "c1", []byte("kubeconfA")))
	got, ok := m.ClientFor("c1")
	require.True(t, ok)
	assert.Equal(t, "kubeconfA", got.(*stubClient).id)
}

func TestManager_ReconnectOnlyOnConfigChange(t *testing.T) {
	m, canceled := newTestManager()
	require.NoError(t, m.Connect(context.Background(), "c1", []byte("A")))
	require.NoError(t, m.Connect(context.Background(), "c1", []byte("A")))
	assert.Empty(t, *canceled, "unchanged kubeconfig must not rebuild")
	require.NoError(t, m.Connect(context.Background(), "c1", []byte("B")))
	assert.Equal(t, []string{"A"}, *canceled, "changed kubeconfig must dispose the old client")
	got, _ := m.ClientFor("c1")
	assert.Equal(t, "B", got.(*stubClient).id)
}

func TestManager_Disconnect(t *testing.T) {
	m, canceled := newTestManager()
	require.NoError(t, m.Connect(context.Background(), "c1", []byte("A")))
	m.Disconnect("c1")
	_, ok := m.ClientFor("c1")
	assert.False(t, ok, "Disconnect must remove the client")
	assert.Equal(t, []string{"A"}, *canceled, "Disconnect must cancel the watch context")
}

// TestManager_ConnectFailureEvictsStaleClient: when the kubeconfig CHANGES and
// the rebuild fails, the previously-cached client (for the now-stale config)
// must be evicted, not left serving against rotated-away credentials.
func TestManager_ConnectFailureEvictsStaleClient(t *testing.T) {
	var canceled []string
	m := NewManager(runtime.NewScheme())
	buildErr := false
	m.newClient = func(_ context.Context, raw []byte, _ *runtime.Scheme) (SelectivelyCachingClient, context.CancelFunc, error) {
		if buildErr {
			return nil, nil, errors.New("build failed")
		}
		id := string(raw)
		return &stubClient{id: id}, func() { canceled = append(canceled, id) }, nil
	}
	require.NoError(t, m.Connect(context.Background(), "c1", []byte("good")))
	_, ok := m.ClientFor("c1")
	require.True(t, ok)

	buildErr = true
	err := m.Connect(context.Background(), "c1", []byte("rotated"))
	require.Error(t, err)
	_, ok = m.ClientFor("c1")
	assert.False(t, ok, "a failed rebuild for a changed kubeconfig must evict the stale client")
	assert.Equal(t, []string{"good"}, canceled, "the stale client's watch context must be cancelled")
}

// TestManager_StartSetsBaseContext: Start records the long-lived context —
// remote clients must derive from a manager-scoped ctx, not a request ctx — and
// disconnects everything when that context is cancelled.
func TestManager_StartSetsBaseContext(t *testing.T) {
	m, canceled := newTestManager()
	require.NoError(t, m.Connect(context.Background(), "c1", []byte("A")))

	assert.Nil(t, m.BaseContext(), "no base context before Start")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = m.Start(ctx); close(done) }()

	require.Eventually(t, func() bool { return m.BaseContext() != nil }, time.Second, time.Millisecond,
		"Start must record the base context")

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
	_, ok := m.ClientFor("c1")
	assert.False(t, ok, "Start must disconnect all clients on shutdown")
	assert.Equal(t, []string{"A"}, *canceled, "shutdown must cancel each client's watch context")
}

func TestManager_ClientForUnknown(t *testing.T) {
	m, _ := newTestManager()
	_, ok := m.ClientFor("nope")
	assert.False(t, ok)
}

func TestManager_Connected(t *testing.T) {
	m, _ := newTestManager()
	require.NoError(t, m.Connect(context.Background(), "a", []byte("kubeconfigA")))
	require.NoError(t, m.Connect(context.Background(), "b", []byte("kubeconfigB")))

	names := m.Connected()
	require.Len(t, names, 2)
	assert.ElementsMatch(t, []string{"a", "b"}, names)

	m.Disconnect("a")
	names = m.Connected()
	assert.Equal(t, []string{"b"}, names)
}
