package constants

import "testing"

func TestLWSNameTruncatesForStatefulSetRevisionLabel(t *testing.T) {
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
			maxLWSNameSuffixLength := maxLabelValueLength - len("-") - len(tt.revisionHash) - len(lwsNamePrefix)

			got := LWSName(tt.componentName)
			revisionLabelValue := got + "-" + tt.revisionHash
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
