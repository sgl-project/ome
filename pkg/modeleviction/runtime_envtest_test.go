package modeleviction

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	lws "sigs.k8s.io/lws/api/leaderworkerset/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/modelusage"
)

// This test starts only a local API server and etcd. No cluster configuration,
// serving pods, model files or real kubelet endpoints are used.
func TestManagerWiresInitialReferencesAndRemovalEvents(t *testing.T) {
	assets := os.Getenv("KUBEBUILDER_ASSETS")
	if assets == "" {
		t.Skip("KUBEBUILDER_ASSETS is required for the local API-server test")
	}
	if _, err := os.Stat(filepath.Join(assets, "kube-apiserver")); err != nil {
		t.Fatal(err)
	}
	t.Run("without InferenceReplica API", func(t *testing.T) { testManagerReferences(t, false) })
	t.Run("with InferenceReplica API", func(t *testing.T) { testManagerReferences(t, true) })
}

func testManagerReferences(t *testing.T, inferenceReplicas bool) {
	environment := &envtest.Environment{CRDs: []*apiextensionsv1.CustomResourceDefinition{
		referenceCRD("BaseModel", "basemodels"), referenceCRD("InferenceService", "inferenceservices"),
	}}
	if inferenceReplicas {
		environment.CRDs = append(environment.CRDs, referenceCRD("InferenceReplica", "inferencereplicas"))
	}
	config, err := environment.Start()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, environment.Stop()) })
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, v1beta1.AddToScheme, lws.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	api, err := client.New(config, client.Options{Scheme: scheme})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, api.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "customer"}}))
	model := &v1beta1.BaseModel{ObjectMeta: metav1.ObjectMeta{Namespace: "customer", Name: "model"}}
	isvc := &v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{Namespace: "customer", Name: "endpoint"}, Spec: v1beta1.InferenceServiceSpec{Model: &v1beta1.ModelRef{Name: "model"}}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "customer", Name: "serving", Labels: map[string]string{constants.InferenceServicePodLabelKey: "endpoint", constants.InferenceServiceBaseModelNameLabelKey: "model"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "test", Image: "test:latest"}}}}
	// A model label alone does not make a Pod an OME workload.
	unrelated := pod.DeepCopy()
	unrelated.Name = "unrelated"
	delete(unrelated.Labels, constants.InferenceServicePodLabelKey)
	// Objects predate startup, including an ISVC without any use claim.
	for _, object := range []client.Object{model, isvc, pod, unrelated} {
		require.NoError(t, api.Create(ctx, object))
	}
	replica := &v1beta1.InferenceReplica{
		ObjectMeta: metav1.ObjectMeta{Namespace: "customer", Name: "replica"},
		Spec:       v1beta1.InferenceReplicaSpec{ModelRef: &v1beta1.ModelRef{Name: "model"}},
	}
	if inferenceReplicas {
		require.NoError(t, api.Create(ctx, replica))
	}
	cacheOptions, err := CacheOptions()
	require.NoError(t, err)
	mgr, err := ctrl.NewManager(config, ctrl.Options{Scheme: scheme, Cache: cacheOptions, Metrics: metricsserver.Options{BindAddress: "0"}, HealthProbeBindAddress: "0"})
	require.NoError(t, err)
	options := DefaultOptions()
	options.NodeSelector = "model-pool=true"
	capacity := capacityFunc(func(context.Context, *corev1.Node) (CapacitySample, error) { return CapacitySample{}, nil })
	require.NoError(t, SetupWithManager(ctx, mgr, capacity, options))
	stopped := make(chan error, 1)
	go func() { stopped <- mgr.Start(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-stopped:
			require.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Error("manager did not stop")
		}
	})
	syncCtx, stopSync := context.WithTimeout(ctx, 10*time.Second)
	defer stopSync()
	require.True(t, mgr.GetCache().WaitForCacheSync(syncCtx))

	// Initial LIST caches the serving Pod, but excludes the unrelated Pod.
	cachedPods := &corev1.PodList{}
	require.NoError(t, mgr.GetCache().List(ctx, cachedPods))
	require.Len(t, cachedPods.Items, 1)
	require.Equal(t, pod.Name, cachedPods.Items[0].Name)

	// WATCH applies the same selector when a Pod enters or leaves its scope.
	unrelated.Labels[constants.InferenceServicePodLabelKey] = "endpoint"
	require.NoError(t, api.Update(ctx, unrelated))
	require.Eventually(t, func() bool {
		return mgr.GetCache().Get(ctx, client.ObjectKeyFromObject(unrelated), &corev1.Pod{}) == nil
	}, 5*time.Second, 10*time.Millisecond)
	delete(unrelated.Labels, constants.InferenceServicePodLabelKey)
	require.NoError(t, api.Update(ctx, unrelated))
	require.Eventually(t, func() bool {
		return apierrors.IsNotFound(mgr.GetCache().Get(ctx, client.ObjectKeyFromObject(unrelated), &corev1.Pod{}))
	}, 5*time.Second, 10*time.Millisecond)

	query := modelusage.NewReferenceQuery(mgr.GetCache(), func() bool { return true }, modelusage.SourceOptions{InferenceReplicas: inferenceReplicas})
	key := types.NamespacedName{Namespace: "customer", Name: "model"}
	referenced, err := query.HasReferences(ctx, key)
	require.NoError(t, err)
	require.True(t, referenced)

	// Removing ISVC alone must preserve protection while its serving Pod remains.
	require.NoError(t, api.Delete(ctx, isvc))
	require.Eventually(t, func() bool {
		return apierrors.IsNotFound(mgr.GetCache().Get(ctx, client.ObjectKeyFromObject(isvc), &v1beta1.InferenceService{}))
	}, 5*time.Second, 10*time.Millisecond)
	referenced, err = query.HasReferences(ctx, key)
	require.NoError(t, err)
	require.True(t, referenced)
	stored := &v1beta1.BaseModel{}
	require.NoError(t, api.Get(ctx, key, stored))
	require.Empty(t, stored.Annotations[constants.ModelUsageHistoryAnnotation])

	require.NoError(t, api.Delete(ctx, pod, client.GracePeriodSeconds(0)))
	require.Eventually(t, func() bool {
		return apierrors.IsNotFound(mgr.GetCache().Get(ctx, client.ObjectKeyFromObject(pod), &corev1.Pod{}))
	}, 5*time.Second, 10*time.Millisecond)
	if inferenceReplicas {
		// An installed InferenceReplica protects the model even after the ISVC
		// and Pod disappear. Its removal must also trigger the history worker.
		referenced, err = query.HasReferences(ctx, key)
		require.NoError(t, err)
		require.True(t, referenced)
		require.NoError(t, api.Get(ctx, key, stored))
		require.Empty(t, stored.Annotations[constants.ModelUsageHistoryAnnotation])
		require.NoError(t, api.Delete(ctx, replica))
	}

	// The final workload removal is delivered through the real watch handler.
	require.Eventually(t, func() bool {
		if err := api.Get(ctx, key, stored); err != nil {
			return false
		}
		return stored.Annotations[constants.ModelUsageHistoryAnnotation] != ""
	}, 5*time.Second, 10*time.Millisecond)
	require.Contains(t, stored.Annotations[constants.ModelUsageHistoryAnnotation], "lastObservedDemandEndedAt")
	require.Empty(t, stored.Annotations["ome.io/artifact-residency"])
}

// Only schema storage is needed; the package does not depend on admission
// webhooks or serving controllers when testing its own watch integration.
func referenceCRD(kind, plural string) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: plural + ".ome.io"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "ome.io", Scope: apiextensionsv1.NamespaceScoped,
			Names:    apiextensionsv1.CustomResourceDefinitionNames{Kind: kind, Plural: plural},
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1beta1", Served: true, Storage: true, Schema: &apiextensionsv1.CustomResourceValidation{OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{Type: "object", XPreserveUnknownFields: ptr.To(true)}}}},
		},
	}
}
