package paging

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// fakeObj is a minimal runtime.Object double so these tests can exercise
// ListAllPaged's control flow without depending on any concrete API type.
type fakeObj struct {
	metav1.TypeMeta
	N string
}

func (f *fakeObj) DeepCopyObject() runtime.Object {
	c := *f
	return &c
}

func obj(n string) runtime.Object { return &fakeObj{N: n} }

func TestListAllPagedDrainsThreePagesFollowingContinueTokens(t *testing.T) {
	pages := [][]runtime.Object{
		{obj("a"), obj("b")},
		{obj("c")},
		{obj("d"), obj("e")},
	}
	tokens := []string{"tok-1", "tok-2", ""} // empty token on the 3rd page ends the list
	var gotOpts []metav1.ListOptions
	call := 0
	page := func(opts metav1.ListOptions) ([]runtime.Object, string, error) {
		gotOpts = append(gotOpts, opts)
		items := pages[call]
		tok := tokens[call]
		call++
		return items, tok, nil
	}

	got, err := ListAllPaged(context.Background(), page)

	require.NoError(t, err)
	require.Len(t, got, 5)
	assert.Equal(t, 3, call, "page should be called exactly once per page")

	// Every request asks for a full chunk...
	for _, o := range gotOpts {
		assert.Equal(t, int64(ChunkSize), o.Limit)
	}
	// ...and each page's continue token becomes the *next* request's
	// Continue -- the first request has none yet.
	assert.Empty(t, gotOpts[0].Continue, "first page has no continue token yet")
	assert.Equal(t, "tok-1", gotOpts[1].Continue)
	assert.Equal(t, "tok-2", gotOpts[2].Continue)
}

func TestListAllPagedPropagatesErrorMidStreamAndStopsPaging(t *testing.T) {
	boom := errors.New("etcd is on fire")
	call := 0
	page := func(opts metav1.ListOptions) ([]runtime.Object, string, error) {
		call++
		if call == 1 {
			return []runtime.Object{obj("a")}, "tok-1", nil
		}
		return nil, "", boom
	}

	got, err := ListAllPaged(context.Background(), page)

	require.ErrorIs(t, err, boom)
	assert.Nil(t, got, "a mid-stream error should not return a partial page")
	assert.Equal(t, 2, call, "paging must stop at the failing page, not retry or continue")
}

func TestListAllPagedEmptyResultStopsAfterOneCall(t *testing.T) {
	call := 0
	page := func(opts metav1.ListOptions) ([]runtime.Object, string, error) {
		call++
		return nil, "", nil
	}

	got, err := ListAllPaged(context.Background(), page)

	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Equal(t, 1, call, "an empty first page with no continue token must not be re-fetched")
}

func TestChunkSizeMatchesKubectlDefault(t *testing.T) {
	// Pin the documented contract (500-item chunks, kubectl parity) so a
	// change here is a deliberate, reviewed decision rather than an
	// accidental drift.
	assert.EqualValues(t, 500, ChunkSize)
}
