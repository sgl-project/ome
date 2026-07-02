package constants

import (
	"strings"
	"testing"
)

func TestLWSNameTruncates(t *testing.T) {
	tests := []struct {
		name              string
		componentName     string
		expectedTruncated bool
	}{
		{
			name:              "short component name",
			componentName:     "my-model-engine",
			expectedTruncated: false,
		},
		{
			name:              "actual long engine component name",
			componentName:     "amaaaaaabgjpxjqamiuior4qamufon2clgneukbxomingadlfcsgq67sicoa-engine",
			expectedTruncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const (
				maxLabelValueLength        = 63
				maxLWSNameLength           = 35
				lwsNamePrefix              = "lws-"
				workerStatefulSetSuffix    = "-0"
				statefulSetRevisionHash    = "75965d8c9f"
				kueueLeaderWorkerSetPrefix = "leaderworkerset-"
				kueueWorkloadHashSuffix    = "-ece5c"
			)

			got := LWSName(tt.componentName)
			workerStatefulSetName := got + workerStatefulSetSuffix
			revisionLabelValue := workerStatefulSetName + "-" + statefulSetRevisionHash
			kueueWorkloadLabelValue := kueueLeaderWorkerSetPrefix + got + workerStatefulSetSuffix + kueueWorkloadHashSuffix
			expected := TruncateNameWithPrefix(tt.componentName, lwsNamePrefix, maxLWSNameLength)

			if len(got) > maxLWSNameLength {
				t.Fatalf("LWSName length = %d, want <= %d: %q", len(got), maxLWSNameLength, got)
			}
			if !strings.HasPrefix(got, lwsNamePrefix) {
				t.Fatalf("LWSName = %q, want prefix %q", got, lwsNamePrefix)
			}
			if tt.expectedTruncated && got == lwsNamePrefix+tt.componentName {
				t.Fatalf("LWSName = %q, want truncated name", got)
			}
			if len(revisionLabelValue) > maxLabelValueLength {
				t.Fatalf("StatefulSet revision label value length = %d, want <= %d: %q", len(revisionLabelValue), maxLabelValueLength, revisionLabelValue)
			}
			if len(kueueWorkloadLabelValue) > maxLabelValueLength {
				t.Fatalf("Kueue workload label value length = %d, want <= %d: %q", len(kueueWorkloadLabelValue), maxLabelValueLength, kueueWorkloadLabelValue)
			}
			if got != expected {
				t.Fatalf("LWSName = %q, want %q", got, expected)
			}
		})
	}
}
