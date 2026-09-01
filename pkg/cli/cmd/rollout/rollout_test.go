package rollout

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	k8stesting "k8s.io/client-go/testing"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"sigs.k8s.io/ome/pkg/client/clientset/versioned"
	omefake "sigs.k8s.io/ome/pkg/client/clientset/versioned/fake"

	omev1beta1 "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/cli/factory"
	reportv1alpha1 "sigs.k8s.io/ome/pkg/cli/report/v1alpha1"
	"sigs.k8s.io/ome/pkg/cli/rolloutprojection"
	"sigs.k8s.io/ome/pkg/constants"
)

func TestStatusRejectsOutputBeforeNamespaceOrClient(t *testing.T) {
	f := &trackingFactory{}
	_, err := execute(t, f, fixedClock(), "status", "chat", "-o", "wide")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "supported: table, json, yaml")
	assert.Zero(t, f.namespaceCalls)
	assert.Zero(t, f.omeCalls)
}

func TestStatusRequiresExactlyOneInferenceServiceBeforeReads(t *testing.T) {
	f := &trackingFactory{}
	_, err := execute(t, f, fixedClock(), "status")

	require.Error(t, err)
	assert.Zero(t, f.namespaceCalls)
	assert.Zero(t, f.omeCalls)
}

func TestStatusRejectsInvalidInferenceServiceNameBeforeReads(t *testing.T) {
	for _, name := range []string{"", "   ", "bad/name", "UpperCase"} {
		t.Run(name, func(t *testing.T) {
			f := &trackingFactory{}
			output, err := execute(t, f, fixedClock(), "status", name)

			require.ErrorIs(t, err, ErrInvalidInferenceServiceName)
			assert.Empty(t, output)
			assert.Zero(t, f.namespaceCalls)
			assert.Zero(t, f.omeCalls)
			if name != "" {
				assert.NotContains(t, err.Error(), name)
			}
		})
	}
}

func TestStatusPerformsExactlyOneInferenceServiceGet(t *testing.T) {
	isvc := minimalInferenceService()
	client := omefake.NewSimpleClientset(isvc)
	f := factory.Static{OME: client, NS: "prod"}

	output, err := execute(t, f, fixedClock(), "status", "chat")
	require.NoError(t, err)
	assert.Equal(t,
		"STATE           REPORTED-STATE   EVIDENCE   EPOCH           GROUP   STRATEGY   GROUP-PHASE   CURRENT-COMPONENT   PREVIOUS-COMPONENT   COMPONENT   COMPONENT-PHASE   STEP   GATE   CAPACITY   TARGET-TRAFFIC   OBSERVED-TRAFFIC   ROLLED-OUT   READY   PREVIOUS   ISSUES\n"+
			"NotConfigured   NotConfigured    Declared   NotApplicable   -       -          -             -                   -                    -           -                 -      -      -          -                -                  -            -       -          -\n",
		output,
	)
	require.Len(t, client.Actions(), 1)
	action := client.Actions()[0]
	assert.Equal(t, "get", action.GetVerb())
	assert.Equal(t, "inferenceservices", action.GetResource().Resource)
	assert.Equal(t, "prod", action.GetNamespace())
}

func TestStatusRejectsUnboundInferenceServiceResponses(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*omev1beta1.InferenceService)
		want   error
	}{
		{name: "returned name mismatch", mutate: func(isvc *omev1beta1.InferenceService) {
			isvc.Name = "other"
		}, want: ErrReturnedInferenceServiceNameMismatch},
		{name: "returned namespace mismatch", mutate: func(isvc *omev1beta1.InferenceService) {
			isvc.Namespace = "other"
		}, want: ErrReturnedInferenceServiceNamespaceMismatch},
		{name: "returned uid absent", mutate: func(isvc *omev1beta1.InferenceService) {
			isvc.UID = ""
		}, want: rolloutprojection.ErrSubjectUIDRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			returned := minimalInferenceService()
			tt.mutate(returned)
			client := omefake.NewSimpleClientset()
			client.PrependReactor("get", "inferenceservices", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, returned.DeepCopy(), nil
			})

			output, err := execute(
				t,
				factory.Static{OME: client, NS: "prod"},
				fixedClock(),
				"status", "chat",
			)

			require.ErrorIs(t, err, tt.want)
			assert.Empty(t, output)
			require.Len(t, client.Actions(), 1)
			assert.Equal(t, "get", client.Actions()[0].GetVerb())
		})
	}
}

func TestStatusWritesExactJSON(t *testing.T) {
	client := omefake.NewSimpleClientset(minimalInferenceService())
	output, err := execute(t, factory.Static{OME: client, NS: "prod"}, fixedClock(), "status", "chat", "-o", "json")

	require.NoError(t, err)
	assert.Equal(t, `{
  "apiVersion": "cli.ome.io/v1alpha1",
  "kind": "RolloutStatusReport",
  "metadata": {
    "namespace": "prod",
    "name": "chat"
  },
  "collectedAt": "2026-08-31T18:30:00Z",
  "sources": [
    {
      "kind": "InferenceService",
      "namespace": "prod",
      "name": "chat",
      "uid": "isvc-uid",
      "generation": 7,
      "evidence": "Observed",
      "collectedAt": "2026-08-31T18:30:00Z"
    }
  ],
  "content": {
    "summary": {
      "state": "NotConfigured",
      "reportedState": "NotConfigured",
      "evidence": "Declared",
      "epoch": "NotApplicable",
      "coordinationReady": "NotApplicable"
    },
    "groups": [],
    "components": [],
    "issues": []
  },
  "warnings": []
}
`, output)
}

func TestStatusWritesExactYAML(t *testing.T) {
	client := omefake.NewSimpleClientset(minimalInferenceService())
	output, err := execute(t, factory.Static{OME: client, NS: "prod"}, fixedClock(), "status", "chat", "-o", "yaml")

	require.NoError(t, err)
	assert.Equal(t, `apiVersion: cli.ome.io/v1alpha1
collectedAt: "2026-08-31T18:30:00Z"
content:
  components: []
  groups: []
  issues: []
  summary:
    coordinationReady: NotApplicable
    epoch: NotApplicable
    evidence: Declared
    reportedState: NotConfigured
    state: NotConfigured
kind: RolloutStatusReport
metadata:
  name: chat
  namespace: prod
sources:
- collectedAt: "2026-08-31T18:30:00Z"
  evidence: Observed
  generation: 7
  kind: InferenceService
  name: chat
  namespace: prod
  uid: isvc-uid
warnings: []
`, output)
}

func TestStatusWritesExactIndependentFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   string
	}{
		{
			name: "table",
			want: "STATE     REPORTED-STATE   EVIDENCE   EPOCH          GROUP   STRATEGY      GROUP-PHASE   CURRENT-COMPONENT   PREVIOUS-COMPONENT   COMPONENT   COMPONENT-PHASE   STEP   GATE   CAPACITY   TARGET-TRAFFIC   OBSERVED-TRAFFIC   ROLLED-OUT   READY   PREVIOUS   ISSUES\n" +
				"Unknown   Succeeded        Reported   Unverifiable   -       Independent   -             -                   -                    engine      Stable            -      -      -          -                -                  -            -       -          EpochUnverifiable\n",
		},
		{
			name: "json", format: "json",
			want: `{
  "apiVersion": "cli.ome.io/v1alpha1",
  "kind": "RolloutStatusReport",
  "metadata": {
    "namespace": "prod",
    "name": "chat"
  },
  "collectedAt": "2026-08-31T18:30:00Z",
  "sources": [
    {
      "kind": "InferenceService",
      "namespace": "prod",
      "name": "chat",
      "uid": "isvc-uid",
      "generation": 7,
      "evidence": "Observed",
      "collectedAt": "2026-08-31T18:30:00Z"
    }
  ],
  "content": {
    "summary": {
      "state": "Unknown",
      "reportedState": "Succeeded",
      "evidence": "Reported",
      "epoch": "Unverifiable",
      "coordinationReady": "NotApplicable"
    },
    "groups": [],
    "components": [
      {
        "type": "engine",
        "strategy": "Independent",
        "phase": "Stable",
        "traffic": []
      }
    ],
    "issues": [
      {
        "code": "EpochUnverifiable"
      }
    ]
  },
  "warnings": [
    {
      "code": "PartialData"
    }
  ]
}
`,
		},
		{
			name: "yaml", format: "yaml",
			want: `apiVersion: cli.ome.io/v1alpha1
collectedAt: "2026-08-31T18:30:00Z"
content:
  components:
  - phase: Stable
    strategy: Independent
    traffic: []
    type: engine
  groups: []
  issues:
  - code: EpochUnverifiable
  summary:
    coordinationReady: NotApplicable
    epoch: Unverifiable
    evidence: Reported
    reportedState: Succeeded
    state: Unknown
kind: RolloutStatusReport
metadata:
  name: chat
  namespace: prod
sources:
- collectedAt: "2026-08-31T18:30:00Z"
  evidence: Observed
  generation: 7
  kind: InferenceService
  name: chat
  namespace: prod
  uid: isvc-uid
warnings:
- code: PartialData
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := omefake.NewSimpleClientset(independentInferenceService())
			args := []string{"status", "chat"}
			if tt.format != "" {
				args = append(args, "-o", tt.format)
			}
			output, err := execute(t, factory.Static{OME: client, NS: "prod"}, fixedClock(), args...)
			require.NoError(t, err)
			assert.Equal(t, tt.want, output)
		})
	}
}

func TestStatusReturnsFriendlyGetError(t *testing.T) {
	output, err := execute(t, factory.Static{OME: omefake.NewSimpleClientset(), NS: "prod"}, fixedClock(), "status", "missing")

	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "not found")
	assert.Empty(t, output)
}

func TestRolloutCommandLocalHelpIsExact(t *testing.T) {
	var output bytes.Buffer
	cmd := NewCmd(factory.Static{NS: "prod"}, genericiooptions.IOStreams{Out: &output, ErrOut: &output})
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"status", "--help"})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, `Show rollout progress for an InferenceService

Usage:
  rollout status INFERENCESERVICE [flags]

Flags:
  -h, --help            help for status
  -o, --output string   Output format: table, json, or yaml (default "table")
`, output.String())
}

func execute(t *testing.T, f factory.Factory, clock reportv1alpha1.Clock, args ...string) (string, error) {
	t.Helper()
	var output bytes.Buffer
	streams := genericiooptions.IOStreams{In: &bytes.Buffer{}, Out: &output, ErrOut: &output}
	cmd := newCmdWithClock(f, streams, clock)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	err := cmd.Execute()
	return output.String(), err
}

func minimalInferenceService() *omev1beta1.InferenceService {
	return &omev1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "prod", Name: "chat", UID: "isvc-uid", Generation: 7,
			ResourceVersion: "SECRET_RESOURCE_VERSION",
			Annotations:     map[string]string{"ome.io/rollout-promote": "SECRET_MAILBOX"},
		},
		Status: omev1beta1.InferenceServiceStatus{
			Status: duckv1.Status{
				ObservedGeneration: 7,
				Annotations:        map[string]string{"secret": "SECRET_STATUS_ANNOTATION"},
			},
			LastRuntimeSyncToken: "SECRET_SYNC_TOKEN",
		},
	}
}

func independentInferenceService() *omev1beta1.InferenceService {
	isvc := minimalInferenceService()
	mode := constants.OMENative
	isvc.Spec.DeploymentMode = &mode
	isvc.Spec.Engine = &omev1beta1.EngineSpec{}
	isvc.Status.Components = map[omev1beta1.ComponentType]omev1beta1.ComponentStatusSpec{
		omev1beta1.EngineComponent: {
			Lifecycle: &omev1beta1.LifecycleStatus{
				CurrentRevision: "chat-engine-aaaaaaaa",
				UpdateRevision:  "chat-engine-aaaaaaaa",
			},
		},
	}
	return isvc
}

func fixedClock() reportv1alpha1.Clock {
	return reportv1alpha1.ClockFunc(func() time.Time {
		return time.Date(2026, time.August, 31, 18, 30, 0, 0, time.UTC)
	})
}

type trackingFactory struct {
	factory.Static
	namespaceCalls int
	omeCalls       int
}

func (f *trackingFactory) Namespace() (string, bool, error) {
	f.namespaceCalls++
	return "", false, errors.New("namespace must not be called")
}

func (f *trackingFactory) OMEClient() (versioned.Interface, error) {
	f.omeCalls++
	return nil, errors.New("OME client must not be called")
}
