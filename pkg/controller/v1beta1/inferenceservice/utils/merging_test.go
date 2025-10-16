package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeMultilineArgs(t *testing.T) {
	tests := []struct {
		name          string
		containerArgs []string
		overrideArgs  []string
		expected      []string
	}{
		{
			name: "Merge multiline args with backslash continuation",
			containerArgs: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0 \
--port=8080 \
--enable-metrics \
--log-requests \
--model-path="$MODEL_PATH" \
--mem-frac=0.9`},
			overrideArgs: []string{`--tp-size=4`},
			expected: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0 \
--port=8080 \
--enable-metrics \
--log-requests \
--model-path="$MODEL_PATH" \
--mem-frac=0.9 \
--tp-size=4`},
		},
		{
			name: "Merge multiline args without trailing backslash",
			containerArgs: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0 \
--mem-frac=0.9`},
			overrideArgs: []string{`--tp-size=8`},
			expected: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0 \
--mem-frac=0.9 \
--tp-size=8`},
		},
		{
			name:          "Empty override args returns container args",
			containerArgs: []string{`python3 -m server`},
			overrideArgs:  []string{},
			expected:      []string{`python3 -m server`},
		},
		{
			name:          "Empty container args returns override args",
			containerArgs: []string{},
			overrideArgs:  []string{`--tp-size=4`},
			expected:      []string{`--tp-size=4`},
		},
		{
			name:          "Single-line args are appended",
			containerArgs: []string{`python3 -m server`},
			overrideArgs:  []string{`--debug`},
			expected:      []string{`python3 -m server`, `--debug`},
		},
		{
			name: "Multiple override args",
			containerArgs: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0`},
			overrideArgs: []string{`--tp-size=4`, `--pp-size=2`},
			expected: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0 \
--tp-size=4
--pp-size=2`},
		},
		{
			name: "Container args with multiple elements",
			containerArgs: []string{`python3 -m server \
--port=8080`, `extra-arg`},
			overrideArgs: []string{`--tp-size=4`},
			expected: []string{`python3 -m server \
--port=8080 \
--tp-size=4`, `extra-arg`},
		},
		{
			name:          "Multiline args with newlines",
			containerArgs: []string{"python3 -m sglang.launch_server\n--host=0.0.0.0\n--port=8080"},
			overrideArgs:  []string{`--tp-size=4`},
			expected:      []string{"python3 -m sglang.launch_server\n--host=0.0.0.0\n--port=8080 \\\n--tp-size=4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeMultilineArgs(tt.containerArgs, tt.overrideArgs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOverrideIntParam(t *testing.T) {
	tests := []struct {
		name          string
		containerArgs []string
		key           string
		value         int64
		expectedArgs  []string
		expectedFound bool
	}{
		{
			name: "Override existing parameter with equals sign",
			containerArgs: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0 \
--tp-size=4 \
--mem-frac=0.9`},
			key:   "--tp-size",
			value: 8,
			expectedArgs: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0 \
--tp-size=8 \
--mem-frac=0.9`},
			expectedFound: true,
		},
		{
			name: "Override existing parameter with space",
			containerArgs: []string{`python3 -m server \
--tp-size 4 \
--mem-frac=0.9`},
			key:   "--tp-size",
			value: 8,
			expectedArgs: []string{`python3 -m server \
--tp-size=8 \
--mem-frac=0.9`},
			expectedFound: true,
		},
		{
			name: "Parameter not found",
			containerArgs: []string{`python3 -m server \
--host=0.0.0.0`},
			key:   "--tp-size",
			value: 8,
			expectedArgs: []string{`python3 -m server \
--host=0.0.0.0`},
			expectedFound: false,
		},
		{
			name:          "Empty container args",
			containerArgs: []string{},
			key:           "--tp-size",
			value:         8,
			expectedArgs:  []string{},
			expectedFound: false,
		},
		{
			name: "Override pipeline parallel size",
			containerArgs: []string{`python3 -m server \
--pp-size=2 \
--tp-size=4`},
			key:   "--pp-size",
			value: 4,
			expectedArgs: []string{`python3 -m server \
--pp-size=4 \
--tp-size=4`},
			expectedFound: true,
		},
		{
			name: "Override tensor-parallel-size (long form)",
			containerArgs: []string{`python3 -m server \
--tensor-parallel-size=4`},
			key:   "--tensor-parallel-size",
			value: 8,
			expectedArgs: []string{`python3 -m server \
--tensor-parallel-size=8`},
			expectedFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := OverrideIntParam(tt.containerArgs, tt.key, tt.value)
			assert.Equal(t, tt.expectedFound, found)
			assert.Equal(t, tt.expectedArgs, result)
		})
	}
}

func TestMergeMultilineArgsEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		containerArgs []string
		overrideArgs  []string
		expected      []string
	}{
		{
			name:          "Nil container args",
			containerArgs: nil,
			overrideArgs:  []string{`--tp-size=4`},
			expected:      []string{`--tp-size=4`},
		},
		{
			name:          "Nil override args",
			containerArgs: []string{`python3 -m server`},
			overrideArgs:  nil,
			expected:      []string{`python3 -m server`},
		},
		{
			name:          "Both nil",
			containerArgs: nil,
			overrideArgs:  nil,
			expected:      nil,
		},
		{
			name:          "Both empty",
			containerArgs: []string{},
			overrideArgs:  []string{},
			expected:      []string{},
		},
		{
			name: "Override with whitespace",
			containerArgs: []string{`python3 -m server \
--port=8080`},
			overrideArgs: []string{`  --tp-size=4  `},
			expected: []string{`python3 -m server \
--port=8080 \
--tp-size=4`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeMultilineArgs(tt.containerArgs, tt.overrideArgs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeMultilineArgsUnion(t *testing.T) {
	tests := []struct {
		name          string
		containerArgs []string
		overrideArgs  []string
		expected      []string
	}{
		{
			name: "Union: duplicate arg not added",
			containerArgs: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0 \
--port=8080 \
--tp-size=4`},
			overrideArgs: []string{`--tp-size=4`},
			expected: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0 \
--port=8080 \
--tp-size=4`},
		},
		{
			name: "Union: new arg added, duplicate ignored",
			containerArgs: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0 \
--tp-size=4`},
			overrideArgs: []string{`--tp-size=4`, `--pp-size=2`},
			expected: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0 \
--tp-size=4 \
--pp-size=2`},
		},
		{
			name: "Union: all new args added",
			containerArgs: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0`},
			overrideArgs: []string{`--tp-size=4`, `--pp-size=2`, `--dp=1`},
			expected: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0 \
--tp-size=4
--pp-size=2
--dp=1`},
		},
		{
			name: "Union: exact match with trailing backslash",
			containerArgs: []string{`python3 -m server \
--enable-metrics \
--log-requests \`},
			overrideArgs: []string{`--enable-metrics`, `--new-flag`},
			expected: []string{`python3 -m server \
--enable-metrics \
--log-requests \
--new-flag`},
		},
		{
			name: "Union: case sensitive comparison",
			containerArgs: []string{`python3 -m server \
--Enable-Metrics`},
			overrideArgs: []string{`--enable-metrics`},
			expected: []string{`python3 -m server \
--Enable-Metrics \
--enable-metrics`},
		},
		{
			name: "Union: arg with different values (key=value format) - override replaces",
			containerArgs: []string{`python3 -m server \
--tp-size=4 \
--port=8080`},
			overrideArgs: []string{`--tp-size=8`, `--host=0.0.0.0`},
			expected: []string{`python3 -m server \
--tp-size=8 \
--port=8080 \
--host=0.0.0.0`},
		},
		{
			name: "Union: empty override args",
			containerArgs: []string{`python3 -m server \
--tp-size=4`},
			overrideArgs: []string{},
			expected: []string{`python3 -m server \
--tp-size=4`},
		},
		{
			name: "Union: whitespace variations - same value keeps existing",
			containerArgs: []string{`python3 -m server \
--tp-size=4  \
  --host=0.0.0.0`},
			overrideArgs: []string{`  --tp-size=4  `, `--new-flag`},
			expected: []string{`python3 -m server \
--tp-size=4  \
  --host=0.0.0.0 \
--new-flag`},
		},
		{
			name: "Union: whitespace variations - different value replaces",
			containerArgs: []string{`python3 -m server \
--tp-size=4  \
  --host=0.0.0.0`},
			overrideArgs: []string{`  --tp-size=8  `, `--new-flag`},
			expected: []string{`python3 -m server \
--tp-size=8 \
  --host=0.0.0.0 \
--new-flag`},
		},
		{
			name: "Union: multiple overrides with replacement",
			containerArgs: []string{`python3 -m server \
--tp-size=4 \
--pp-size=2 \
--port=8080`},
			overrideArgs: []string{`python3 -m server`, `--tp-size=8`, `--pp-size=4`, `--new-flag`},
			expected: []string{`python3 -m server \
--tp-size=8 \
--pp-size=4 \
--port=8080 \
--new-flag`},
		},
		{
			name: "Union: args with space separator (--key value format)",
			containerArgs: []string{`python3 -m server \
--tp-size 4 \
--port 8080`},
			overrideArgs: []string{`--tp-size 8`, `--host 0.0.0.0`},
			expected: []string{`python3 -m server \
--tp-size 8 \
--port 8080 \
--host 0.0.0.0`},
		},
		{
			name: "Union: flag without value",
			containerArgs: []string{`python3 -m server \
--enable-metrics \
--port=8080`},
			overrideArgs: []string{`--enable-metrics`, `--debug`},
			expected: []string{`python3 -m server \
--enable-metrics \
--port=8080 \
--debug`},
		},
		{
			name: "Union: multi-line override with command and flag replacement",
			containerArgs: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0 \
--port=8080 \
--enable-metrics \
--log-requests \
--model-path="$MODEL_PATH" \
--tp-size=4 \
--mem-frac=0.9`},
			overrideArgs: []string{`python3 -m sglang.launch_server \
--tp-size=2`},
			expected: []string{`python3 -m sglang.launch_server \
--host=0.0.0.0 \
--port=8080 \
--enable-metrics \
--log-requests \
--model-path="$MODEL_PATH" \
--tp-size=2 \
--mem-frac=0.9`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeMultilineArgs(tt.containerArgs, tt.overrideArgs)
			assert.Equal(t, tt.expected, result)
		})
	}
}
