package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParsePropertyIntervals(t *testing.T) {
	input := strings.NewReader(`# ignored
0600..0605 ; Prepend # comment
261D ; Emoji_Modifier_Base
094D ; InCB; Linker # comment
`)

	got, err := parsePropertyIntervals(input)
	if err != nil {
		t.Fatalf("parsePropertyIntervals() error = %v", err)
	}
	want := []propertyInterval{
		{first: 0x0600, last: 0x0605, property: "Prepend"},
		{first: 0x261D, last: 0x261D, property: "Emoji_Modifier_Base"},
		{first: 0x094D, last: 0x094D, property: "InCB=Linker"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePropertyIntervals() = %#v, want %#v", got, want)
	}
}

func TestMergeIntervalsSortsAndCoalesces(t *testing.T) {
	got := mergeIntervals([]codePointInterval{
		{first: 0x20, last: 0x22},
		{first: 0x10, last: 0x10},
		{first: 0x11, last: 0x14},
		{first: 0x21, last: 0x24},
	})
	want := []codePointInterval{
		{first: 0x10, last: 0x14},
		{first: 0x20, last: 0x24},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mergeIntervals() = %#v, want %#v", got, want)
	}
}

func TestSelectNamedIntervalsSortsAndCoalescesByProperty(t *testing.T) {
	got := selectNamedIntervals(
		[]propertyInterval{
			{first: 0x20, last: 0x20, property: "B"},
			{first: 0x11, last: 0x12, property: "A"},
			{first: 0x10, last: 0x10, property: "A"},
			{first: 0x21, last: 0x21, property: "B"},
			{first: 0x30, last: 0x30, property: "ignored"},
		},
		map[string]string{"A": "propertyA", "B": "propertyB"},
	)
	want := []namedInterval{
		{first: 0x10, last: 0x12, name: "propertyA"},
		{first: 0x20, last: 0x21, name: "propertyB"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selectNamedIntervals() = %#v, want %#v", got, want)
	}
}

func TestNormalizeGraphemeTestsRetainsEveryDefinition(t *testing.T) {
	input := strings.NewReader(`# GraphemeBreakTest-17.0.0.txt
# comment
÷ 0061 × 0308 ÷ # trailing

÷ 0062 ÷
`)

	got, count, err := normalizeGraphemeTests(input)
	if err != nil {
		t.Fatalf("normalizeGraphemeTests() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("normalizeGraphemeTests() count = %d, want 2", count)
	}
	want := "÷ 0061 × 0308 ÷\n÷ 0062 ÷\n"
	if got != want {
		t.Fatalf("normalizeGraphemeTests() = %q, want %q", got, want)
	}
}

func TestParseOptionsKeepsLocalSourceOverrides(t *testing.T) {
	got, err := parseOptions([]string{"-emoji-data-source", "/tmp/emoji-data.txt"})
	if err != nil {
		t.Fatalf("parseOptions() error = %v", err)
	}
	if got.sourcePaths["emoji-data-source"] != "/tmp/emoji-data.txt" {
		t.Fatalf(
			"parseOptions() emoji source = %q, want %q",
			got.sourcePaths["emoji-data-source"],
			"/tmp/emoji-data.txt",
		)
	}
}

func TestLoadSourceVerifiesPinnedHashWithoutNetwork(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(path, []byte("source"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	spec := sourceSpec{sha256: "41cf6794ba4200b839c53531555f0f3998df4cbb01a4d5cb0b94e3ca5e23947d"}

	got, err := loadSource(context.Background(), spec, path)
	if err != nil {
		t.Fatalf("loadSource() error = %v", err)
	}
	if string(got) != "source" {
		t.Fatalf("loadSource() = %q, want %q", got, "source")
	}

	spec.sha256 = strings.Repeat("0", 64)
	if _, err := loadSource(context.Background(), spec, path); err == nil {
		t.Fatal("loadSource() error = nil, want a digest mismatch")
	}
}
