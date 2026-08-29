package v1beta1

import (
	"encoding/json"
	"testing"
)

func TestInferenceReplicaStatusDenseJSONUnchanged(t *testing.T) {
	status := InferenceReplicaStatus{
		ObservedGeneration: 7,
		InstanceStatuses: []OMENativeInstanceStatus{{
			Index: 3,
			Phase: OMENativeInstanceReady,
		}},
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal dense status: %v", err)
	}

	want := `{"observedGeneration":7,"instanceStatuses":[{"index":3,"phase":"Ready"}]}`
	if string(data) != want {
		t.Fatalf("dense status JSON changed:\n want %s\n  got %s", want, data)
	}
}

func TestInferenceReplicaStatusColumnarJSON(t *testing.T) {
	encoding := InstanceStatusEncodingColumnarV2
	admitted := "0-2"
	activeOrdinalOne := "2"
	status := InferenceReplicaStatus{
		InstanceStatusEncoding: &encoding,
		InstanceStatusColumns: &InstanceStatusColumns{
			Members:  "0-2",
			RowOrder: []int32{2, 0, 1},
			Phases: []InstanceStatusPhaseGroup{{
				Value:   OMENativeInstanceReady,
				Indexes: "0-2",
			}},
			RunningRevisions: []InstanceStatusStringGroup{{
				Value:   "example-engine-a1b2c3d4",
				Indexes: "0-2",
			}},
			TargetRevisions: []InstanceStatusStringGroup{{
				Value:   "example-engine-e5f6a7b8",
				Indexes: "2",
			}},
			Incarnations: []InstanceStatusInt64Group{{
				Value:   2,
				Indexes: "2",
			}},
			PodCounts: []InstanceStatusCountGroup{{
				Value:   8,
				Indexes: "0-2",
			}},
			ServingPodCounts: []InstanceStatusCountGroup{{
				Value:   8,
				Indexes: "0-2",
			}},
			AvailablePodCounts: []InstanceStatusCountGroup{{
				Value:   8,
				Indexes: "0-2",
			}},
			Admitted:         &admitted,
			ActiveOrdinalOne: &activeOrdinalOne,
			Entries: []InstanceStatusColumnEntry{{
				Index: 2,
				LastFailure: &InstanceTermination{
					PodName: "example-engine-2",
					Reason:  "Error",
				},
			}},
		},
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal columnar status: %v", err)
	}

	want := `{"instanceStatusEncoding":"ColumnarV2","instanceStatusColumns":{"members":"0-2","rowOrder":[2,0,1],"phases":[{"value":"Ready","indexes":"0-2"}],"runningRevisions":[{"value":"example-engine-a1b2c3d4","indexes":"0-2"}],"targetRevisions":[{"value":"example-engine-e5f6a7b8","indexes":"2"}],"incarnations":[{"value":2,"indexes":"2"}],"podCounts":[{"value":8,"indexes":"0-2"}],"servingPodCounts":[{"value":8,"indexes":"0-2"}],"availablePodCounts":[{"value":8,"indexes":"0-2"}],"admitted":"0-2","activeOrdinalOne":"2","entries":[{"index":2,"lastFailure":{"podName":"example-engine-2","reason":"Error","time":null}}]}}`
	if string(data) != want {
		t.Fatalf("columnar status JSON changed:\n want %s\n  got %s", want, data)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("unmarshal columnar status: %v", err)
	}
	if _, ok := fields["instanceStatuses"]; ok {
		t.Fatal("columnar status unexpectedly contains instanceStatuses")
	}
	if _, ok := fields["instanceStatusEncoding"]; !ok {
		t.Fatal("columnar status is missing instanceStatusEncoding")
	}
	if _, ok := fields["instanceStatusColumns"]; !ok {
		t.Fatal("columnar status is missing instanceStatusColumns")
	}

	var decoded InferenceReplicaStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode typed columnar status: %v", err)
	}
	if decoded.InstanceStatusEncoding == nil || *decoded.InstanceStatusEncoding != InstanceStatusEncodingColumnarV2 {
		t.Fatalf("decoded encoding = %v, want ColumnarV2", decoded.InstanceStatusEncoding)
	}
	if decoded.InstanceStatusColumns == nil || decoded.InstanceStatusColumns.Members != "0-2" {
		t.Fatalf("decoded columns = %#v, want members 0-2", decoded.InstanceStatusColumns)
	}
}

func TestInferenceReplicaStatusColumnsDeepCopyDoesNotAlias(t *testing.T) {
	encoding := InstanceStatusEncodingColumnarV2
	admitted := "0-1"
	exitCode := int32(143)
	original := &InferenceReplica{
		Status: InferenceReplicaStatus{
			InstanceStatusEncoding: &encoding,
			InstanceStatusColumns: &InstanceStatusColumns{
				Members:  "0-1",
				RowOrder: []int32{1, 0},
				Phases: []InstanceStatusPhaseGroup{{
					Value:   OMENativeInstanceReady,
					Indexes: "0-1",
				}},
				Admitted: &admitted,
				Entries: []InstanceStatusColumnEntry{{
					Index: 1,
					Operation: &InstanceOperation{
						ID:              "operation-1",
						Type:            InstanceOperationMigrate,
						Step:            "WaitReady",
						HintTargetNodes: []string{"node-a"},
					},
					LastFailure: &InstanceTermination{
						PodName:  "example-engine-1",
						ExitCode: &exitCode,
					},
				}},
			},
		},
	}

	clone := original.DeepCopy()
	*clone.Status.InstanceStatusEncoding = "changed"
	clone.Status.InstanceStatusColumns.Members = "9"
	clone.Status.InstanceStatusColumns.RowOrder[0] = 9
	clone.Status.InstanceStatusColumns.Phases[0].Indexes = "9"
	*clone.Status.InstanceStatusColumns.Admitted = "9"
	clone.Status.InstanceStatusColumns.Entries[0].Operation.HintTargetNodes[0] = "node-b"
	*clone.Status.InstanceStatusColumns.Entries[0].LastFailure.ExitCode = 1

	if got := *original.Status.InstanceStatusEncoding; got != InstanceStatusEncodingColumnarV2 {
		t.Errorf("encoding aliases clone: got %q", got)
	}
	if got := original.Status.InstanceStatusColumns.Members; got != "0-1" {
		t.Errorf("members alias clone: got %q", got)
	}
	if got := original.Status.InstanceStatusColumns.RowOrder[0]; got != 1 {
		t.Errorf("rowOrder aliases clone: got %d", got)
	}
	if got := original.Status.InstanceStatusColumns.Phases[0].Indexes; got != "0-1" {
		t.Errorf("phases alias clone: got %q", got)
	}
	if got := *original.Status.InstanceStatusColumns.Admitted; got != "0-1" {
		t.Errorf("admitted aliases clone: got %q", got)
	}
	if got := original.Status.InstanceStatusColumns.Entries[0].Operation.HintTargetNodes[0]; got != "node-a" {
		t.Errorf("operation aliases clone: got %q", got)
	}
	if got := *original.Status.InstanceStatusColumns.Entries[0].LastFailure.ExitCode; got != 143 {
		t.Errorf("lastFailure aliases clone: got %d", got)
	}
}
