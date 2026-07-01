package constants

import "testing"

func TestLWSNameTruncates(t *testing.T) {
	tests := []struct {
		name          string
		componentName string
		revisionHash  string
	}{
		{
			name:          "actual long engine component name",
			componentName: "amaaaaaabgjpxjqamiuior4qamufon2clgneukbxomingadlfcsgq67sicoa-engine",
			revisionHash:  "75965d8c9f",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxLabelValueLength := 63
			lwsNamePrefix := "lws-"
			workerStatefulSetSuffix := "-0"
			maxLWSNameSuffixLength := maxLabelValueLength - len(workerStatefulSetSuffix) - len("-") - len(tt.revisionHash) - len(lwsNamePrefix)

			got := LWSName(tt.componentName)
			workerStatefulSetName := got + workerStatefulSetSuffix
			revisionLabelValue := workerStatefulSetName + "-" + tt.revisionHash
			expected := lwsNamePrefix + tt.componentName[len(tt.componentName)-maxLWSNameSuffixLength:]

			if len(revisionLabelValue) > maxLabelValueLength {
				t.Fatalf("StatefulSet revision label value length = %d, want <= %d: %q", len(revisionLabelValue), maxLabelValueLength, revisionLabelValue)
			}
			if got != expected {
				t.Fatalf("LWSName = %q, want %q", got, expected)
			}
		})
	}
}
