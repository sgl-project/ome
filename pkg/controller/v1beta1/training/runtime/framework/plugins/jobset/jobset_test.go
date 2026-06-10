package jobset_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	trainingruntime "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime"
	jobsetplugin "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime/framework/plugins/jobset"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/utils"
	testing2 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/testing"
)

const testTrainJobName = "test-job"

func TestBuildReturnsCreateCandidateWhenJobSetExists(t *testing.T) {
	ctx := context.Background()
	trainJob := testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, testTrainJobName).
		UID("uid").
		Suspend(false).
		Obj()
	existingJobSet := testing2.MakeJobSetWrapper(metav1.NamespaceDefault, testTrainJobName).
		Suspend(true).
		Obj()

	plugin, fakeClient := newJobSetPlugin(t, existingJobSet)

	got, err := plugin.Build(ctx, runtimeJobTemplate(), runtimeInfo(), trainJob)
	if err != nil {
		t.Fatalf("Build() returned unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("Build() returned nil, want JobSet create candidate")
	}

	gotJobSet, ok := got.(*jobsetv1alpha2.JobSet)
	if !ok {
		t.Fatalf("Build() returned %T, want *jobsetv1alpha2.JobSet", got)
	}
	if gotJobSet.Name != utils.GetShortTrainJobName(trainJob.Name) {
		t.Fatalf("JobSet name = %q, want %q", gotJobSet.Name, utils.GetShortTrainJobName(trainJob.Name))
	}
	if gotJobSet.ResourceVersion != "" {
		t.Fatalf("JobSet resourceVersion = %q, want empty create candidate", gotJobSet.ResourceVersion)
	}
	if gotJobSet.Spec.Suspend == nil || *gotJobSet.Spec.Suspend {
		t.Fatalf("JobSet suspend = %v, want false", gotJobSet.Spec.Suspend)
	}

	fetchedJobSet := &jobsetv1alpha2.JobSet{}
	if err := fakeClient.Get(ctx, client.ObjectKey{Name: testTrainJobName, Namespace: metav1.NamespaceDefault}, fetchedJobSet); err != nil {
		t.Fatalf("failed to fetch existing JobSet: %v", err)
	}
	if fetchedJobSet.Spec.Suspend == nil || !*fetchedJobSet.Spec.Suspend {
		t.Fatalf("existing JobSet suspend = %v, want unchanged true", fetchedJobSet.Spec.Suspend)
	}
}

func newJobSetPlugin(t *testing.T, objs ...client.Object) (*jobsetplugin.JobSet, client.Client) {
	t.Helper()
	clientBuilder := testing2.NewClientBuilder()
	if len(objs) > 0 {
		clientBuilder = clientBuilder.WithObjects(objs...)
	}
	fakeClient := clientBuilder.Build()
	plugin, err := jobsetplugin.New(context.Background(), fakeClient, nil)
	if err != nil {
		t.Fatalf("failed to create JobSet plugin: %v", err)
	}
	jobSetPlugin, ok := plugin.(*jobsetplugin.JobSet)
	if !ok {
		t.Fatalf("New() returned %T, want *jobset.JobSet", plugin)
	}
	return jobSetPlugin, fakeClient
}

func runtimeJobTemplate() *jobsetv1alpha2.JobSet {
	return testing2.MakeJobSetWrapper(metav1.NamespaceDefault, testTrainJobName).Obj()
}

func runtimeInfo() *trainingruntime.Info {
	return &trainingruntime.Info{
		Labels: map[string]string{
			"runtime-label": "runtime-value",
		},
		Annotations: map[string]string{
			"runtime-annotation": "runtime-value",
		},
		Trainer: trainingruntime.Trainer{
			NumNodes: ptr.To[int32](1),
			Env: []corev1.EnvVar{
				{Name: "INFO_ENV", Value: "info-value"},
			},
		},
		Scheduler: &trainingruntime.Scheduler{
			PodLabels: map[string]string{},
		},
	}
}
