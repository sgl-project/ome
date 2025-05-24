package core

import (
	"context"
	"testing"

	testing2 "github.com/sgl-project/sgl-ome/pkg/testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	jobsetconsts "sigs.k8s.io/jobset/pkg/constants"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework"
	fwkplugins "github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework/plugins"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework/plugins/coscheduling"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework/plugins/jobset"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework/plugins/mpi"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework/plugins/plainml"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework/plugins/torch"
)

// TODO: We should introduce mock plugins and use plugins in this framework testing.
// After we migrate the actual plugins to mock one for testing data,
// we can delegate the actual plugin testing to each plugin directories, and implement detailed unit testing.

func TestNew(t *testing.T) {
	cases := map[string]struct {
		registry                                                               fwkplugins.Registry
		emptyCoSchedulingIndexerTrainingRuntimeContainerRuntimeClassKey        bool
		emptyCoSchedulingIndexerClusterTrainingRuntimeContainerRuntimeClassKey bool
		wantFramework                                                          *Framework
		wantError                                                              error
	}{
		"positive case": {
			registry: fwkplugins.NewRegistry(),
			wantFramework: &Framework{
				registry: fwkplugins.NewRegistry(),
				plugins: map[string]framework.Plugin{
					coscheduling.Name: &coscheduling.CoScheduling{},
					mpi.Name:          &mpi.MPI{},
					plainml.Name:      &plainml.PlainML{},
					torch.Name:        &torch.Torch{},
					jobset.Name:       &jobset.JobSet{},
				},
				enforceMLPlugins: []framework.EnforceMLPolicyPlugin{
					&mpi.MPI{},
					&plainml.PlainML{},
					&torch.Torch{},
				},
				enforcePodGroupPolicyPlugins: []framework.EnforcePodGroupPolicyPlugin{
					&coscheduling.CoScheduling{},
				},
				customValidationPlugins: []framework.CustomValidationPlugin{
					&mpi.MPI{},
					&torch.Torch{},
				},
				watchExtensionPlugins: []framework.WatchExtensionPlugin{
					&coscheduling.CoScheduling{},
					&jobset.JobSet{},
				},
				componentBuilderPlugins: []framework.ComponentBuilderPlugin{
					&coscheduling.CoScheduling{},
					&jobset.JobSet{},
				},
				terminalConditionPlugins: []framework.TerminalConditionPlugin{
					&jobset.JobSet{},
				},
			},
		},
		"indexer key for trainingRuntime and runtimeClass is an empty": {
			registry: fwkplugins.Registry{
				coscheduling.Name: coscheduling.New,
			},
			emptyCoSchedulingIndexerTrainingRuntimeContainerRuntimeClassKey: true,
			wantError: coscheduling.ErrorCanNotSetupTrainingRuntimeRuntimeClassIndexer,
		},
		"indexer key for clusterTrainingRuntime and runtimeClass is an empty": {
			registry: fwkplugins.Registry{
				coscheduling.Name: coscheduling.New,
			},
			emptyCoSchedulingIndexerClusterTrainingRuntimeContainerRuntimeClassKey: true,
			wantError: coscheduling.ErrorCanNotSetupClusterTrainingRuntimeRuntimeClassIndexer,
		},
	}
	cmpOpts := []cmp.Option{
		cmp.AllowUnexported(Framework{}),
		cmpopts.IgnoreUnexported(coscheduling.CoScheduling{}, mpi.MPI{}, plainml.PlainML{}, torch.Torch{}, jobset.JobSet{}),
		cmpopts.IgnoreFields(coscheduling.CoScheduling{}, "client"),
		cmpopts.IgnoreFields(jobset.JobSet{}, "client"),
		cmpopts.IgnoreTypes(apiruntime.Scheme{}, meta.DefaultRESTMapper{}, fwkplugins.Registry{}),
		cmpopts.SortMaps(func(a, b string) bool { return a < b }),
		cmpopts.SortSlices(func(a, b framework.Plugin) bool { return a.Name() < b.Name() }),
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			if tc.emptyCoSchedulingIndexerTrainingRuntimeContainerRuntimeClassKey {
				originTrainingRuntimeRuntimeKey := coscheduling.TrainingRuntimeContainerRuntimeClassKey
				coscheduling.TrainingRuntimeContainerRuntimeClassKey = ""
				t.Cleanup(func() {
					coscheduling.TrainingRuntimeContainerRuntimeClassKey = originTrainingRuntimeRuntimeKey
				})
			}
			if tc.emptyCoSchedulingIndexerClusterTrainingRuntimeContainerRuntimeClassKey {
				originClusterTrainingRuntimeKey := coscheduling.ClusterTrainingRuntimeContainerRuntimeClassKey
				coscheduling.ClusterTrainingRuntimeContainerRuntimeClassKey = ""
				t.Cleanup(func() {
					coscheduling.ClusterTrainingRuntimeContainerRuntimeClassKey = originClusterTrainingRuntimeKey
				})
			}
			clientBuilder := testing2.NewClientBuilder()
			fwk, err := New(ctx, clientBuilder.Build(), tc.registry, testing2.AsIndex(clientBuilder))
			if diff := cmp.Diff(tc.wantError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected errors (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantFramework, fwk, cmpOpts...); len(diff) != 0 {
				t.Errorf("Unexpected framework (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestRunEnforceMLPolicyPlugins(t *testing.T) {
	cases := map[string]struct {
		registry        fwkplugins.Registry
		runtimeInfo     *runtime.Info
		trainJob        *omev1beta1.TrainingJob
		wantRuntimeInfo *runtime.Info
		wantError       error
	}{
		"plainml MLPolicy is applied to runtime.Info, TrainJob doesn't have numNodes": {
			registry: fwkplugins.NewRegistry(),
			runtimeInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: &omev1beta1.MLPolicy{
						NumNodes: ptr.To[int32](100),
					},
				},
				Scheduler: &runtime.Scheduler{
					TotalRequests: map[string]runtime.TotalResourceRequest{
						constants.JobInitializer: {Replicas: 1},
						constants.JobTrainerNode: {Replicas: 10},
					},
				},
			},
			trainJob: &omev1beta1.TrainingJob{
				Spec: omev1beta1.TrainingJobSpec{},
			},
			wantRuntimeInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: &omev1beta1.MLPolicy{
						NumNodes: ptr.To[int32](100),
					},
				},
				Trainer: runtime.Trainer{
					NumNodes: ptr.To[int32](100),
				},
				Scheduler: &runtime.Scheduler{
					TotalRequests: map[string]runtime.TotalResourceRequest{
						constants.JobInitializer: {Replicas: 1},
						constants.JobTrainerNode: {Replicas: 100},
					},
				},
			},
		},
		"plainml MLPolicy is applied to runtime.Info, TrainJob has numNodes": {
			registry: fwkplugins.NewRegistry(),
			runtimeInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: &omev1beta1.MLPolicy{
						NumNodes: ptr.To[int32](100),
					},
				},
				Scheduler: &runtime.Scheduler{
					TotalRequests: map[string]runtime.TotalResourceRequest{
						constants.JobInitializer: {Replicas: 1},
						constants.JobTrainerNode: {Replicas: 10},
					},
				},
			},
			trainJob: &omev1beta1.TrainingJob{
				Spec: omev1beta1.TrainingJobSpec{
					Trainer: &omev1beta1.TrainerSpec{
						NumNodes: ptr.To[int32](30),
					},
				},
			},
			wantRuntimeInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: &omev1beta1.MLPolicy{
						NumNodes: ptr.To[int32](100),
					},
				},
				Trainer: runtime.Trainer{
					NumNodes: ptr.To[int32](30),
				},
				Scheduler: &runtime.Scheduler{
					TotalRequests: map[string]runtime.TotalResourceRequest{
						constants.JobInitializer: {Replicas: 1},
						constants.JobTrainerNode: {Replicas: 30},
					},
				},
			},
		},
		"registry is empty": {
			runtimeInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{
					TotalRequests: map[string]runtime.TotalResourceRequest{
						constants.JobInitializer: {Replicas: 1},
						constants.JobTrainerNode: {Replicas: 10},
					},
				},
			},
			wantRuntimeInfo: &runtime.Info{
				Scheduler: &runtime.Scheduler{
					TotalRequests: map[string]runtime.TotalResourceRequest{
						constants.JobInitializer: {Replicas: 1},
						constants.JobTrainerNode: {Replicas: 10},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			clientBuilder := testing2.NewClientBuilder()

			fwk, err := New(ctx, clientBuilder.Build(), tc.registry, testing2.AsIndex(clientBuilder))
			if err != nil {
				t.Fatal(err)
			}
			err = fwk.RunEnforceMLPolicyPlugins(tc.runtimeInfo, tc.trainJob)
			if diff := cmp.Diff(tc.wantError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error (-want,+got): %s", diff)
			}
			if diff := cmp.Diff(tc.wantRuntimeInfo, tc.runtimeInfo, cmpopts.EquateEmpty()); len(diff) != 0 {
				t.Errorf("Unexpected runtime.Info (-want,+got): %s", diff)
			}
		})
	}
}

func TestRunEnforcePodGroupPolicyPlugins(t *testing.T) {
	cases := map[string]struct {
		registry        fwkplugins.Registry
		runtimeInfo     *runtime.Info
		trainJob        *omev1beta1.TrainingJob
		wantRuntimeInfo *runtime.Info
		wantError       error
	}{
		"coscheduling plugin is applied to runtime.Info": {
			registry: fwkplugins.NewRegistry(),
			runtimeInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &omev1beta1.PodGroupPolicy{
						CoschedulingPodGroupPolicyConfig: &omev1beta1.CoschedulingPodGroupPolicyConfig{
							ScheduleTimeoutSeconds: ptr.To[int32](99),
						},
					},
				},
				Scheduler: &runtime.Scheduler{},
			},
			trainJob: &omev1beta1.TrainingJob{ObjectMeta: metav1.ObjectMeta{Name: "test-job", Namespace: metav1.NamespaceDefault}},
			wantRuntimeInfo: &runtime.Info{
				RuntimePolicy: runtime.RuntimePolicy{
					PodGroupPolicy: &omev1beta1.PodGroupPolicy{
						CoschedulingPodGroupPolicyConfig: &omev1beta1.CoschedulingPodGroupPolicyConfig{
							ScheduleTimeoutSeconds: ptr.To[int32](99),
						},
					},
				},
				Scheduler: &runtime.Scheduler{
					PodLabels: map[string]string{
						schedulerpluginsv1alpha1.PodGroupLabel: "test-job",
					},
				},
			},
		},
		"an empty registry": {
			trainJob:        &omev1beta1.TrainingJob{ObjectMeta: metav1.ObjectMeta{Name: "test-job", Namespace: metav1.NamespaceDefault}},
			runtimeInfo:     &runtime.Info{},
			wantRuntimeInfo: &runtime.Info{},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			clientBuilder := testing2.NewClientBuilder()

			fwk, err := New(ctx, clientBuilder.Build(), tc.registry, testing2.AsIndex(clientBuilder))
			if err != nil {
				t.Fatal(err)
			}
			err = fwk.RunEnforcePodGroupPolicyPlugins(tc.runtimeInfo, tc.trainJob)
			if diff := cmp.Diff(tc.wantError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error (-want,+got): %s", diff)
			}
			if diff := cmp.Diff(tc.wantRuntimeInfo, tc.runtimeInfo); len(diff) != 0 {
				t.Errorf("Unexpected runtime.Info (-want,+got): %s", diff)
			}
		})
	}
}

func TestRunCustomValidationPlugins(t *testing.T) {
	cases := map[string]struct {
		registry     fwkplugins.Registry
		oldObj       *omev1beta1.TrainingJob
		newObj       *omev1beta1.TrainingJob
		wantWarnings admission.Warnings
		wantError    field.ErrorList
	}{
		// Need to implement more detail testing after we implement custom validator in any plugins.
		"there are not any custom validations": {
			registry: fwkplugins.NewRegistry(),
			oldObj:   testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Obj(),
			newObj:   testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Obj(),
		},
		"an empty registry": {
			oldObj: testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Obj(),
			newObj: testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "test").Obj(),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			clientBuildr := testing2.NewClientBuilder()

			fwk, err := New(ctx, clientBuildr.Build(), tc.registry, testing2.AsIndex(clientBuildr))
			if err != nil {
				t.Fatal(err)
			}
			warnings, errs := fwk.RunCustomValidationPlugins(tc.oldObj, tc.newObj)
			if diff := cmp.Diff(tc.wantWarnings, warnings, cmpopts.SortSlices(func(a, b string) bool { return a < b })); len(diff) != 0 {
				t.Errorf("Unexpected warninigs (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantError, errs, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); len(diff) != 0 {
				t.Errorf("Unexpected error (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestRunComponentBuilderPlugins(t *testing.T) {
	resRequests := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("1"),
		corev1.ResourceMemory: resource.MustParse("4Gi"),
	}

	cases := map[string]struct {
		registry           fwkplugins.Registry
		runtimeInfo        *runtime.Info
		trainJob           *omev1beta1.TrainingJob
		runtimeJobTemplate client.Object
		wantRuntimeInfo    *runtime.Info
		wantObjs           []client.Object
		wantError          error
	}{
		"succeeded to build PodGroup and JobSet with NumNodes from TrainJob": {
			registry:           fwkplugins.NewRegistry(),
			runtimeJobTemplate: testing2.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").DeepCopy(),
			runtimeInfo: &runtime.Info{
				Labels: map[string]string{
					schedulerpluginsv1alpha1.PodGroupLabel: "test-job",
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: &omev1beta1.MLPolicy{
						NumNodes: ptr.To[int32](10),
					},
					PodGroupPolicy: &omev1beta1.PodGroupPolicy{
						CoschedulingPodGroupPolicyConfig: &omev1beta1.CoschedulingPodGroupPolicyConfig{
							ScheduleTimeoutSeconds: ptr.To[int32](300),
						},
					},
				},
				Trainer: runtime.Trainer{
					NumNodes: ptr.To[int32](10),
				},
				Scheduler: &runtime.Scheduler{
					TotalRequests: map[string]runtime.TotalResourceRequest{
						constants.JobInitializer: {
							Replicas:    1,
							PodRequests: resRequests,
						},
						constants.JobTrainerNode: {
							Replicas:    1,
							PodRequests: resRequests,
						},
					},
				},
			},
			trainJob: testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "test-job").
				UID("uid").
				Trainer(
					testing2.MakeTrainJobTrainerWrapper().
						NumNodes(100).
						Container("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
						Obj(),
				).
				Obj(),
			wantRuntimeInfo: &runtime.Info{
				Labels: map[string]string{
					schedulerpluginsv1alpha1.PodGroupLabel: "test-job",
				},
				RuntimePolicy: runtime.RuntimePolicy{
					MLPolicy: &omev1beta1.MLPolicy{
						NumNodes: ptr.To[int32](10),
					},
					PodGroupPolicy: &omev1beta1.PodGroupPolicy{
						CoschedulingPodGroupPolicyConfig: &omev1beta1.CoschedulingPodGroupPolicyConfig{
							ScheduleTimeoutSeconds: ptr.To[int32](300),
						},
					},
				},
				Trainer: runtime.Trainer{
					NumNodes: ptr.To[int32](100),
				},
				Scheduler: &runtime.Scheduler{
					PodLabels: map[string]string{schedulerpluginsv1alpha1.PodGroupLabel: "test-job"},
					TotalRequests: map[string]runtime.TotalResourceRequest{
						constants.JobInitializer: {
							Replicas:    1,
							PodRequests: resRequests,
						},
						constants.JobTrainerNode: {
							Replicas:    100,         // Replicas is taken from TrainJob NumNodes.
							PodRequests: resRequests, // TODO: Add support for TrainJob ResourcesPerNode in TotalRequests.
						},
					},
				},
			},
			wantObjs: []client.Object{
				testing2.MakeSchedulerPluginsPodGroup(metav1.NamespaceDefault, "test-job").
					SchedulingTimeout(300).
					MinMember(101). // 101 replicas = 100 Trainer nodes + 1 Initializer.
					MinResources(corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("101"), // 1 CPU and 4Gi per replica.
						corev1.ResourceMemory: resource.MustParse("404Gi"),
					}).
					ControllerReference(omev1beta1.SchemeGroupVersion.WithKind("TrainingJob"), "test-job", "uid").
					Obj(),
				testing2.MakeJobSetWrapper(metav1.NamespaceDefault, "test-job").
					ControllerReference(omev1beta1.SchemeGroupVersion.WithKind("TrainingJob"), "test-job", "uid").
					NumNodes(100).
					Labels(schedulerpluginsv1alpha1.PodGroupLabel, "test-job").
					LabelsTrainer(schedulerpluginsv1alpha1.PodGroupLabel, "test-job").
					ContainerTrainer("test:trainjob", []string{"trainjob"}, []string{"trainjob"}, resRequests).
					Obj(),
			},
		},
		"an empty registry": {},
	}
	cmpOpts := []cmp.Option{
		cmpopts.SortSlices(func(a, b client.Object) bool {
			return a.GetObjectKind().GroupVersionKind().String() < b.GetObjectKind().GroupVersionKind().String()
		}),
		cmpopts.EquateEmpty(),
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			clientBuilder := testing2.NewClientBuilder()

			fwk, err := New(ctx, clientBuilder.Build(), tc.registry, testing2.AsIndex(clientBuilder))
			if err != nil {
				t.Fatal(err)
			}

			if err = fwk.RunEnforcePodGroupPolicyPlugins(tc.runtimeInfo, tc.trainJob); err != nil {
				t.Fatal(err)
			}
			if err = fwk.RunEnforceMLPolicyPlugins(tc.runtimeInfo, tc.trainJob); err != nil {
				t.Fatal(err)
			}
			objs, err := fwk.RunComponentBuilderPlugins(ctx, tc.runtimeJobTemplate, tc.runtimeInfo, tc.trainJob)
			if diff := cmp.Diff(tc.wantError, err, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected errors (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantRuntimeInfo, tc.runtimeInfo); len(diff) != 0 {
				t.Errorf("Unexpected runtime.Info (-want,+got)\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantObjs, objs, cmpOpts...); len(diff) != 0 {
				t.Errorf("Unexpected objects (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestWatchExtensionPlugins(t *testing.T) {
	cases := map[string]struct {
		registry    fwkplugins.Registry
		wantPlugins []framework.WatchExtensionPlugin
	}{
		"coscheding and jobset are performed": {
			registry: fwkplugins.NewRegistry(),
			wantPlugins: []framework.WatchExtensionPlugin{
				&coscheduling.CoScheduling{},
				&jobset.JobSet{},
			},
		},
		"an empty registry": {
			wantPlugins: nil,
		},
	}
	cmpOpts := []cmp.Option{
		cmpopts.SortSlices(func(a, b framework.Plugin) bool { return a.Name() < b.Name() }),
		cmpopts.IgnoreUnexported(coscheduling.CoScheduling{}, jobset.JobSet{}),
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			clientBuilder := testing2.NewClientBuilder()

			fwk, err := New(ctx, clientBuilder.Build(), tc.registry, testing2.AsIndex(clientBuilder))
			if err != nil {
				t.Fatal(err)
			}
			plugins := fwk.WatchExtensionPlugins()
			if diff := cmp.Diff(tc.wantPlugins, plugins, cmpOpts...); len(diff) != 0 {
				t.Errorf("Unexpected plugins (-want,+got):\n%s", diff)
			}
		})
	}
}

type fakeTerminalConditionPlugin struct{}

var _ framework.TerminalConditionPlugin = (*fakeTerminalConditionPlugin)(nil)

func newFakeTerminalConditionPlugin(context.Context, client.Client, client.FieldIndexer) (framework.Plugin, error) {
	return &fakeTerminalConditionPlugin{}, nil
}

const fakeTerminalConditionPluginName = "fake"

func (f fakeTerminalConditionPlugin) Name() string { return fakeTerminalConditionPluginName }
func (f fakeTerminalConditionPlugin) TerminalCondition(context.Context, *omev1beta1.TrainingJob) (*metav1.Condition, error) {
	return nil, nil
}

func TestTerminalConditionPlugins(t *testing.T) {
	cases := map[string]struct {
		registry      fwkplugins.Registry
		trainJob      *omev1beta1.TrainingJob
		jobSet        *jobsetv1alpha2.JobSet
		wantCondition *metav1.Condition
		wantError     error
	}{
		"jobSet has not been finalized, yet": {
			registry: fwkplugins.NewRegistry(),
			trainJob: testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "testing").
				Obj(),
			jobSet: testing2.MakeJobSetWrapper(metav1.NamespaceDefault, "testing").
				Conditions(metav1.Condition{
					Type:    string(jobsetv1alpha2.JobSetSuspended),
					Reason:  jobsetconsts.JobSetSuspendedReason,
					Message: jobsetconsts.JobSetSuspendedMessage,
					Status:  metav1.ConditionFalse,
				}).
				Obj(),
		},
		"succeeded to obtain completed terminal condition": {
			registry: fwkplugins.NewRegistry(),
			trainJob: testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "testing").
				Obj(),
			jobSet: testing2.MakeJobSetWrapper(metav1.NamespaceDefault, "testing").
				Conditions(metav1.Condition{
					Type:    string(jobsetv1alpha2.JobSetCompleted),
					Reason:  jobsetconsts.AllJobsCompletedReason,
					Message: jobsetconsts.AllJobsCompletedMessage,
					Status:  metav1.ConditionTrue,
				}).
				Obj(),
			wantCondition: &metav1.Condition{
				Type:    omev1beta1.TrainJobComplete,
				Reason:  jobsetconsts.AllJobsCompletedReason,
				Message: jobsetconsts.AllJobsCompletedMessage,
				Status:  metav1.ConditionTrue,
			},
		},
		"succeeded to obtain failed terminal condition": {
			registry: fwkplugins.NewRegistry(),
			trainJob: testing2.MakeTrainJobWrapper(metav1.NamespaceDefault, "testing").
				Obj(),
			jobSet: testing2.MakeJobSetWrapper(metav1.NamespaceDefault, "testing").
				Conditions(metav1.Condition{
					Type:    string(jobsetv1alpha2.JobSetFailed),
					Reason:  jobsetconsts.FailedJobsReason,
					Message: jobsetconsts.FailedJobsMessage,
					Status:  metav1.ConditionTrue,
				}).
				Obj(),
			wantCondition: &metav1.Condition{
				Type:    omev1beta1.TrainJobFailed,
				Reason:  jobsetconsts.FailedJobsReason,
				Message: jobsetconsts.FailedJobsMessage,
				Status:  metav1.ConditionTrue,
			},
		},
		"failed to obtain any terminal condition due to multiple terminalCondition plugin": {
			registry: fwkplugins.Registry{
				jobset.Name:                     jobset.New,
				fakeTerminalConditionPluginName: newFakeTerminalConditionPlugin,
			},
			wantError: errorTooManyTerminalConditionPlugin,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			clientBuilder := testing2.NewClientBuilder()
			if tc.jobSet != nil {
				clientBuilder = clientBuilder.WithObjects(tc.jobSet)
			}
			fwk, err := New(ctx, clientBuilder.Build(), tc.registry, testing2.AsIndex(clientBuilder))
			if err != nil {
				t.Fatal(err)
			}
			gotCond, gotErr := fwk.RunTerminalConditionPlugins(ctx, tc.trainJob)
			if diff := cmp.Diff(tc.wantError, gotErr, cmpopts.EquateErrors()); len(diff) != 0 {
				t.Errorf("Unexpected error (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantCondition, gotCond); len(diff) != 0 {
				t.Errorf("Unexpected terminal condition (-want,+got):\n%s", diff)
			}
		})
	}
}
