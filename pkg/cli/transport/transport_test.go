package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"

	omev1beta1 "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestNewRejectsInvalidRESTConfig(t *testing.T) {
	t.Parallel()

	client, err := New(nil)
	assert.Nil(t, client)
	require.EqualError(t, err, "transport: REST config is nil")

	client, err = New(&rest.Config{Host: "://invalid"})
	assert.Nil(t, client)
	require.Error(t, err)
}

func TestNewDoesNotMutateRESTConfig(t *testing.T) {
	t.Parallel()

	groupVersion := schema.GroupVersion{Group: "example.test", Version: "v9"}
	config := &rest.Config{
		Host:      "https://cluster.example.test",
		APIPath:   "/custom",
		UserAgent: "transport-test",
		ContentConfig: rest.ContentConfig{
			GroupVersion:       &groupVersion,
			ContentType:        "application/custom",
			AcceptContentTypes: "application/custom,application/json",
		},
	}

	client, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Same(t, &groupVersion, config.GroupVersion)
	assert.Equal(t, "/custom", config.APIPath)
	assert.Equal(t, "application/custom", config.ContentType)
	assert.Equal(t, "application/custom,application/json", config.AcceptContentTypes)
	assert.Equal(t, "transport-test", config.UserAgent)
}

func TestJSONPatchPreservesWireContractForServerDryRun(t *testing.T) {
	t.Parallel()

	type recordedRequest struct {
		method      string
		path        string
		rawQuery    string
		contentType string
		body        string
	}
	requests := make(chan recordedRequest, 1)
	wantResponse := []byte(" { \"kind\" : \"InferenceService\", \"apiVersion\" : \"ome.io/v1beta1\" }\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		requests <- recordedRequest{
			method:      r.Method,
			path:        r.URL.Path,
			rawQuery:    r.URL.RawQuery,
			contentType: r.Header.Get("Content-Type"),
			body:        string(body),
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(wantResponse)
	}))
	t.Cleanup(server.Close)

	client, err := New(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	patch := []byte(`[{"op":"test","path":"/metadata/resourceVersion","value":"42"},{"op":"add","path":"/metadata/annotations/ome.io~1paused","value":"true"}]`)

	got, err := client.JSONPatch(context.Background(), Resource{
		Namespace: "team-a",
		Resource:  "inferenceservices",
		Name:      "demo",
	}, patch, JSONPatchOptions{DryRun: true})
	require.NoError(t, err)
	assert.Equal(t, wantResponse, got)

	request := <-requests
	assert.Equal(t, http.MethodPatch, request.method)
	assert.Equal(t, "/apis/ome.io/v1beta1/namespaces/team-a/inferenceservices/demo", request.path)
	assert.Equal(t, "dryRun=All", request.rawQuery)
	assert.Equal(t, "application/json-patch+json", request.contentType)
	assert.Equal(t, string(patch), request.body)
}

func TestJSONPatchOmitsDryRunByDefault(t *testing.T) {
	t.Parallel()

	requests := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Clone(context.Background())
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(server.Close)

	client, err := New(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	_, err = client.JSONPatch(context.Background(), Resource{
		Namespace: "team-a",
		Resource:  "inferenceservices",
		Name:      "demo",
	}, []byte(`[]`), JSONPatchOptions{})
	require.NoError(t, err)

	request := <-requests
	assert.Empty(t, request.URL.Query().Get("dryRun"))
	assert.Empty(t, request.URL.RawQuery)
}

func TestJSONPatchDecodesKubernetesStatusError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"apiVersion":"v1","kind":"Status","status":"Failure","message":"the object has been modified","reason":"Conflict","details":{"name":"demo","group":"ome.io","kind":"InferenceService"},"code":409}`)
	}))
	t.Cleanup(server.Close)

	client, err := New(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	got, err := client.JSONPatch(context.Background(), Resource{
		Namespace: "team-a",
		Resource:  "inferenceservices",
		Name:      "demo",
	}, []byte(`[]`), JSONPatchOptions{})

	assert.Nil(t, got)
	require.Error(t, err)
	assert.True(t, apierrors.IsConflict(err), "error must retain Kubernetes reason: %v", err)
	var statusError *apierrors.StatusError
	require.True(t, errors.As(err, &statusError), "error type = %T", err)
	require.NotNil(t, statusError.ErrStatus.Details)
	assert.Equal(t, "demo", statusError.ErrStatus.Details.Name)
	assert.Equal(t, "ome.io", statusError.ErrStatus.Details.Group)
	assert.Equal(t, "InferenceService", statusError.ErrStatus.Details.Kind)
}

func TestWatchStreamsTypedOMEEvents(t *testing.T) {
	t.Parallel()

	requests := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Clone(context.Background())
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"type":"ADDED","object":{"apiVersion":"ome.io/v1beta1","kind":"InferenceReplica","metadata":{"namespace":"team-a","name":"demo-engine","resourceVersion":"74"}}}`+"\n")
	}))
	t.Cleanup(server.Close)

	client, err := New(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	watcher, err := client.Watch(context.Background(), Collection{
		Namespace: "team-a",
		Resource:  "inferencereplicas",
	}, metav1.ListOptions{
		ResourceVersion:     "73",
		FieldSelector:       "metadata.name=demo-engine",
		AllowWatchBookmarks: true,
	})
	require.NoError(t, err)
	t.Cleanup(watcher.Stop)

	select {
	case event, ok := <-watcher.ResultChan():
		require.True(t, ok)
		assert.Equal(t, watch.Added, event.Type)
		replica, ok := event.Object.(*omev1beta1.InferenceReplica)
		require.True(t, ok, "event object type = %T", event.Object)
		assert.Equal(t, "team-a", replica.Namespace)
		assert.Equal(t, "demo-engine", replica.Name)
		assert.Equal(t, "74", replica.ResourceVersion)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for watch event")
	}

	request := <-requests
	assert.Equal(t, http.MethodGet, request.Method)
	assert.Equal(t, "/apis/ome.io/v1beta1/namespaces/team-a/inferencereplicas", request.URL.Path)
	query := request.URL.Query()
	assert.Equal(t, "true", query.Get("watch"))
	assert.Equal(t, "73", query.Get("resourceVersion"))
	assert.Equal(t, "metadata.name=demo-engine", query.Get("fieldSelector"))
	assert.Equal(t, "true", query.Get("allowWatchBookmarks"))
}

func TestWatchUsesClusterScopeAndTimeoutOptions(t *testing.T) {
	t.Parallel()

	requests := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Clone(context.Background())
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"type":"ADDED","object":{"apiVersion":"ome.io/v1beta1","kind":"AcceleratorQuota","metadata":{"name":"team-a","resourceVersion":"75"}}}`+"\n")
	}))
	t.Cleanup(server.Close)

	client, err := New(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	timeoutSeconds := int64(7)
	watcher, err := client.Watch(context.Background(), Collection{
		Resource: "acceleratorquotas",
	}, metav1.ListOptions{TimeoutSeconds: &timeoutSeconds})
	require.NoError(t, err)
	t.Cleanup(watcher.Stop)

	select {
	case event, ok := <-watcher.ResultChan():
		require.True(t, ok)
		quota, ok := event.Object.(*omev1beta1.AcceleratorQuota)
		require.True(t, ok, "event object type = %T", event.Object)
		assert.Empty(t, quota.Namespace)
		assert.Equal(t, "team-a", quota.Name)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for cluster-scoped watch event")
	}

	request := <-requests
	assert.Equal(t, "/apis/ome.io/v1beta1/acceleratorquotas", request.URL.Path)
	query := request.URL.Query()
	assert.Equal(t, "true", query.Get("watch"))
	assert.Equal(t, "7", query.Get("timeoutSeconds"))
	assert.Equal(t, "7s", query.Get("timeout"))
}

func TestWatchCancellationClosesHTTPStream(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	serverCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "response writer cannot flush", http.StatusInternalServerError)
			return
		}
		flusher.Flush()
		close(started)
		<-r.Context().Done()
		close(serverCanceled)
	}))
	t.Cleanup(server.Close)

	client, err := New(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	watcher, err := client.Watch(ctx, Collection{
		Namespace: "team-a",
		Resource:  "inferencereplicas",
	}, metav1.ListOptions{})
	require.NoError(t, err)
	t.Cleanup(watcher.Stop)

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for watch stream to start")
	}
	cancel()

	select {
	case <-serverCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("canceling the watch context did not cancel the HTTP request")
	}
	deadline := time.After(2 * time.Second)
	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return
			}
			// client-go may surface the canceled read as one terminal error
			// event before closing the channel.
			assert.Equal(t, watch.Error, event.Type)
		case <-deadline:
			t.Fatal("watch result channel did not close after cancellation")
		}
	}
}

func TestGetInferenceReplicaScaleUsesScaleSubresource(t *testing.T) {
	t.Parallel()

	requests := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Clone(context.Background())
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"apiVersion":"autoscaling/v1","kind":"Scale","metadata":{"namespace":"team-a","name":"demo-engine","resourceVersion":"81"},"spec":{"replicas":3},"status":{"replicas":2,"selector":"ome.io/component=engine"}}`)
	}))
	t.Cleanup(server.Close)

	client, err := New(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	scale, err := client.GetInferenceReplicaScale(
		context.Background(), "team-a", "demo-engine", metav1.GetOptions{ResourceVersion: "80"},
	)
	require.NoError(t, err)
	assert.Equal(t, "team-a", scale.Namespace)
	assert.Equal(t, "demo-engine", scale.Name)
	assert.Equal(t, "81", scale.ResourceVersion)
	assert.Equal(t, int32(3), scale.Spec.Replicas)
	assert.Equal(t, int32(2), scale.Status.Replicas)
	assert.Equal(t, "ome.io/component=engine", scale.Status.Selector)

	request := <-requests
	assert.Equal(t, http.MethodGet, request.Method)
	assert.Equal(t, "/apis/ome.io/v1beta1/namespaces/team-a/inferencereplicas/demo-engine/scale", request.URL.Path)
	assert.Equal(t, "80", request.URL.Query().Get("resourceVersion"))
}

func TestUpdateInferenceReplicaScaleUsesDryRunScaleSubresource(t *testing.T) {
	t.Parallel()

	type recordedRequest struct {
		method      string
		path        string
		rawQuery    string
		contentType string
		body        []byte
	}
	requests := make(chan recordedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		requests <- recordedRequest{
			method:      r.Method,
			path:        r.URL.Path,
			rawQuery:    r.URL.RawQuery,
			contentType: r.Header.Get("Content-Type"),
			body:        body,
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"apiVersion":"autoscaling/v1","kind":"Scale","metadata":{"namespace":"team-a","name":"demo-engine","resourceVersion":"82"},"spec":{"replicas":5},"status":{"replicas":2}}`)
	}))
	t.Cleanup(server.Close)

	client, err := New(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	desired := &autoscalingv1.Scale{
		TypeMeta: metav1.TypeMeta{APIVersion: "autoscaling/v1", Kind: "Scale"},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "team-a",
			Name:            "demo-engine",
			ResourceVersion: "81",
		},
		Spec: autoscalingv1.ScaleSpec{Replicas: 5},
	}
	scale, err := client.UpdateInferenceReplicaScale(
		context.Background(), "team-a", "demo-engine", desired,
		metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}},
	)
	require.NoError(t, err)
	assert.Equal(t, "82", scale.ResourceVersion)
	assert.Equal(t, int32(5), scale.Spec.Replicas)
	assert.Equal(t, int32(2), scale.Status.Replicas)

	request := <-requests
	assert.Equal(t, http.MethodPut, request.method)
	assert.Equal(t, "/apis/ome.io/v1beta1/namespaces/team-a/inferencereplicas/demo-engine/scale", request.path)
	assert.Equal(t, "dryRun=All", request.rawQuery)
	assert.Equal(t, "application/json", request.contentType)
	assert.JSONEq(t, `{"apiVersion":"autoscaling/v1","kind":"Scale","metadata":{"namespace":"team-a","name":"demo-engine","resourceVersion":"81"},"spec":{"replicas":5},"status":{"replicas":0}}`, string(request.body))
}
