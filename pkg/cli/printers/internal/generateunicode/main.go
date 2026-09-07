// Command generateunicode regenerates the pinned Unicode property tables used
// by the terminal table printer. It is intentionally standard-library-only so
// `go generate` does not add a tool dependency to kubectl-ome.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"go/format"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const unicodeVersion = "17.0.0"

type sourceSpec struct {
	flagName string
	url      string
	sha256   string
}

var sourceSpecs = []sourceSpec{
	{
		flagName: "grapheme-property-source",
		url:      "https://www.unicode.org/Public/17.0.0/ucd/auxiliary/GraphemeBreakProperty.txt",
		sha256:   "d6b51d1d2ae5c33b451b7ed994b48f1f4dc62b2272a5831e7fd418514a6bae89",
	},
	{
		flagName: "emoji-data-source",
		url:      "https://www.unicode.org/Public/17.0.0/ucd/emoji/emoji-data.txt",
		sha256:   "2cb2bb9455cda83e8481541ecf5b6dfda66a3bb89efa3fa7c5297eccf607b72b",
	},
	{
		flagName: "derived-core-properties-source",
		url:      "https://www.unicode.org/Public/17.0.0/ucd/DerivedCoreProperties.txt",
		sha256:   "24c7fed1195c482faaefd5c1e7eb821c5ee1fb6de07ecdbaa64b56a99da22c08",
	},
	{
		flagName: "east-asian-width-source",
		url:      "https://www.unicode.org/Public/17.0.0/ucd/EastAsianWidth.txt",
		sha256:   "ea7ce50f3444a050333448dffef1cadd9325af55cbb764b4a2280faf52170a33",
	},
	{
		flagName: "grapheme-break-test-source",
		url:      "https://www.unicode.org/Public/17.0.0/ucd/auxiliary/GraphemeBreakTest.txt",
		sha256:   "e2d134d2c52919bace503ebb6a551c1855fe1a1faec18478c78fff254a1793ec",
	},
}

type options struct {
	output             string
	graphemeTestOutput string
	sourcePaths        map[string]string
}

type propertyInterval struct {
	first    rune
	last     rune
	property string
}

type codePointInterval struct {
	first rune
	last  rune
}

type namedInterval struct {
	first rune
	last  rune
	name  string
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(parent context.Context, arguments []string) error {
	configuration, err := parseOptions(arguments)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()

	sources := make(map[string][]byte, len(sourceSpecs))
	for _, spec := range sourceSpecs {
		data, loadErr := loadSource(ctx, spec, configuration.sourcePaths[spec.flagName])
		if loadErr != nil {
			return fmt.Errorf("load %s: %w", spec.flagName, loadErr)
		}
		sources[spec.flagName] = data
	}

	properties, err := renderProperties(sources)
	if err != nil {
		return err
	}
	if err := writeGeneratedFile(configuration.output, properties); err != nil {
		return fmt.Errorf("write properties: %w", err)
	}

	tests, count, err := normalizeGraphemeTests(bytes.NewReader(sources["grapheme-break-test-source"]))
	if err != nil {
		return fmt.Errorf("normalize grapheme tests: %w", err)
	}
	if count != 766 {
		return fmt.Errorf("normalize grapheme tests: got %d definitions, want 766", count)
	}
	fixture := graphemeTestHeader() + tests
	if err := writeGeneratedFile(configuration.graphemeTestOutput, []byte(fixture)); err != nil {
		return fmt.Errorf("write grapheme test fixture: %w", err)
	}
	return nil
}

func parseOptions(arguments []string) (options, error) {
	configuration := options{sourcePaths: map[string]string{}}
	flags := flag.NewFlagSet("generateunicode", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&configuration.output, "output", "unicode_properties.go", "generated Go output")
	flags.StringVar(
		&configuration.graphemeTestOutput,
		"grapheme-test-output",
		filepath.Join("testdata", "GraphemeBreakTest-17.0.0.txt"),
		"normalized grapheme conformance fixture",
	)
	for _, spec := range sourceSpecs {
		name := spec.flagName
		flags.Func(name, "read the pinned source from a local file instead of downloading it", func(value string) error {
			configuration.sourcePaths[name] = value
			return nil
		})
	}
	if err := flags.Parse(arguments); err != nil {
		return options{}, err
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	return configuration, nil
}

func loadSource(ctx context.Context, spec sourceSpec, path string) ([]byte, error) {
	var data []byte
	var err error
	if path != "" {
		data, err = os.ReadFile(path)
	} else {
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, spec.url, nil)
		if requestErr != nil {
			return nil, requestErr
		}
		response, requestErr := http.DefaultClient.Do(request)
		if requestErr != nil {
			return nil, requestErr
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GET %s: %s", spec.url, response.Status)
		}
		data, err = io.ReadAll(io.LimitReader(response.Body, 16<<20))
	}
	if err != nil {
		return nil, err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	if digest != spec.sha256 {
		return nil, fmt.Errorf("SHA-256 = %s, want %s", digest, spec.sha256)
	}
	return data, nil
}

func parsePropertyIntervals(reader io.Reader) ([]propertyInterval, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	intervals := []propertyInterval{}
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		definition := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
		if definition == "" {
			continue
		}
		fields := strings.Split(definition, ";")
		if len(fields) < 2 {
			return nil, fmt.Errorf("line %d: expected a semicolon-delimited property", lineNumber)
		}
		first, last, err := parseCodePointRange(strings.TrimSpace(fields[0]))
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		property := strings.TrimSpace(fields[1])
		if property == "InCB" {
			if len(fields) < 3 {
				return nil, fmt.Errorf("line %d: InCB record has no value", lineNumber)
			}
			property += "=" + strings.TrimSpace(fields[2])
		}
		intervals = append(intervals, propertyInterval{first: first, last: last, property: property})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return intervals, nil
}

func parseCodePointRange(value string) (rune, rune, error) {
	parts := strings.Split(value, "..")
	if len(parts) > 2 {
		return 0, 0, fmt.Errorf("invalid code point range %q", value)
	}
	first, err := strconv.ParseInt(parts[0], 16, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("parse code point %q: %w", parts[0], err)
	}
	last := first
	if len(parts) == 2 {
		last, err = strconv.ParseInt(parts[1], 16, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("parse code point %q: %w", parts[1], err)
		}
	}
	if first > last || first < 0 || last > 0x10ffff {
		return 0, 0, fmt.Errorf("invalid code point range %q", value)
	}
	return rune(first), rune(last), nil
}

func intervalsForProperty(intervals []propertyInterval, properties ...string) []codePointInterval {
	allowed := make(map[string]struct{}, len(properties))
	for _, property := range properties {
		allowed[property] = struct{}{}
	}
	result := []codePointInterval{}
	for _, interval := range intervals {
		if _, ok := allowed[interval.property]; ok {
			result = append(result, codePointInterval{first: interval.first, last: interval.last})
		}
	}
	return result
}

func mergeIntervals(intervals []codePointInterval) []codePointInterval {
	if len(intervals) == 0 {
		return nil
	}
	ordered := append([]codePointInterval(nil), intervals...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].first == ordered[j].first {
			return ordered[i].last < ordered[j].last
		}
		return ordered[i].first < ordered[j].first
	})
	result := []codePointInterval{ordered[0]}
	for _, interval := range ordered[1:] {
		last := &result[len(result)-1]
		if interval.first <= last.last+1 {
			if interval.last > last.last {
				last.last = interval.last
			}
			continue
		}
		result = append(result, interval)
	}
	return result
}

func selectNamedIntervals(intervals []propertyInterval, names map[string]string) []namedInterval {
	selected := []namedInterval{}
	for _, interval := range intervals {
		name, ok := names[interval.property]
		if !ok {
			continue
		}
		selected = append(selected, namedInterval{first: interval.first, last: interval.last, name: name})
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].first == selected[j].first {
			return selected[i].last < selected[j].last
		}
		return selected[i].first < selected[j].first
	})
	if len(selected) == 0 {
		return nil
	}

	result := []namedInterval{selected[0]}
	for _, interval := range selected[1:] {
		last := &result[len(result)-1]
		if interval.name == last.name && interval.first == last.last+1 {
			last.last = interval.last
			continue
		}
		result = append(result, interval)
	}
	return result
}

func normalizeGraphemeTests(reader io.Reader) (string, int, error) {
	var output strings.Builder
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	count := 0
	for scanner.Scan() {
		definition := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
		if definition == "" {
			continue
		}
		output.WriteString(definition)
		output.WriteByte('\n')
		count++
	}
	if err := scanner.Err(); err != nil {
		return "", 0, err
	}
	return output.String(), count, nil
}

func renderProperties(sources map[string][]byte) ([]byte, error) {
	grapheme, err := parsePropertyIntervals(bytes.NewReader(sources["grapheme-property-source"]))
	if err != nil {
		return nil, fmt.Errorf("parse GraphemeBreakProperty.txt: %w", err)
	}
	emoji, err := parsePropertyIntervals(bytes.NewReader(sources["emoji-data-source"]))
	if err != nil {
		return nil, fmt.Errorf("parse emoji-data.txt: %w", err)
	}
	derived, err := parsePropertyIntervals(bytes.NewReader(sources["derived-core-properties-source"]))
	if err != nil {
		return nil, fmt.Errorf("parse DerivedCoreProperties.txt: %w", err)
	}
	eastAsianWidth, err := parsePropertyIntervals(bytes.NewReader(sources["east-asian-width-source"]))
	if err != nil {
		return nil, fmt.Errorf("parse EastAsianWidth.txt: %w", err)
	}

	graphemeNames := map[string]string{
		"Prepend": "graphemePrepend", "CR": "graphemeCR", "LF": "graphemeLF",
		"Control": "graphemeControl", "Extend": "graphemeExtend",
		"Regional_Indicator": "graphemeRegionalIndicator", "SpacingMark": "graphemeSpacingMark",
		"L": "graphemeL", "V": "graphemeV", "T": "graphemeT", "LV": "graphemeLV",
		"LVT": "graphemeLVT", "ZWJ": "graphemeZWJ",
	}
	indicNames := map[string]string{
		"InCB=Linker": "indicLinker", "InCB=Consonant": "indicConsonant", "InCB=Extend": "indicExtend",
	}

	var output strings.Builder
	writePropertiesHeader(&output)
	writePropertyTypes(&output)
	writeNamedPropertyIntervals(&output, "graphemePropertyRanges", "graphemePropertyRange", grapheme, graphemeNames)
	writeCodePointIntervals(&output, "extendedPictographicRanges", mergeIntervals(
		intervalsForProperty(emoji, "Extended_Pictographic"),
	))
	writeCodePointIntervals(&output, "emojiModifierBaseRanges", mergeIntervals(
		intervalsForProperty(emoji, "Emoji_Modifier_Base"),
	))
	doubleWidth := append(
		intervalsForProperty(eastAsianWidth, "W", "F"),
		intervalsForProperty(emoji, "Emoji_Presentation")...,
	)
	writeCodePointIntervals(&output, "doubleWidthRanges", mergeIntervals(doubleWidth))
	writeNamedPropertyIntervals(&output, "indicPropertyRanges", "indicPropertyRange", derived, indicNames)

	formatted, err := format.Source([]byte(output.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated properties: %w", err)
	}
	return formatted, nil
}

func writePropertiesHeader(output *strings.Builder) {
	output.WriteString("// Code generated from pinned Unicode 17.0.0 property data. DO NOT EDIT.\n//\n")
	output.WriteString("// Sources and SHA-256 digests:\n")
	for _, spec := range sourceSpecs[:4] {
		fmt.Fprintf(output, "//\n//\t%s\n//\tSHA-256: %s\n", spec.url, spec.sha256)
	}
	output.WriteString("//\n// Unicode data is copyright Unicode, Inc. and distributed under Unicode\n")
	output.WriteString("// License V3; see UNICODE_LICENSE.txt.\npackage printers\n\n")
}

func writePropertyTypes(output *strings.Builder) {
	output.WriteString(`type graphemeProperty uint8

const (
	graphemeOther graphemeProperty = iota
	graphemePrepend
	graphemeCR
	graphemeLF
	graphemeControl
	graphemeExtend
	graphemeRegionalIndicator
	graphemeSpacingMark
	graphemeL
	graphemeV
	graphemeT
	graphemeLV
	graphemeLVT
	graphemeZWJ
)

type indicProperty uint8

const (
	indicNone indicProperty = iota
	indicLinker
	indicConsonant
	indicExtend
)

type graphemePropertyRange struct {
	first    rune
	last     rune
	property graphemeProperty
}

type indicPropertyRange struct {
	first    rune
	last     rune
	property indicProperty
}

type runeInterval struct {
	first rune
	last  rune
}

`)
}

func writeNamedPropertyIntervals(
	output *strings.Builder,
	variable string,
	typeName string,
	intervals []propertyInterval,
	names map[string]string,
) {
	fmt.Fprintf(output, "var %s = []%s{\n", variable, typeName)
	for _, interval := range selectNamedIntervals(intervals, names) {
		fmt.Fprintf(output, "\t{%s, %s, %s},\n", codePoint(interval.first), codePoint(interval.last), interval.name)
	}
	output.WriteString("}\n\n")
}

func writeCodePointIntervals(output *strings.Builder, variable string, intervals []codePointInterval) {
	fmt.Fprintf(output, "var %s = []runeInterval{\n", variable)
	for _, interval := range intervals {
		fmt.Fprintf(output, "\t{%s, %s},\n", codePoint(interval.first), codePoint(interval.last))
	}
	output.WriteString("}\n\n")
}

func codePoint(value rune) string {
	return fmt.Sprintf("0x%04X", value)
}

func graphemeTestHeader() string {
	spec := sourceSpecs[4]
	return fmt.Sprintf(
		"# Derived from Unicode %s GraphemeBreakTest.txt.\n"+
			"# Source: %s\n"+
			"# Source SHA-256: %s\n"+
			"# Licensed under Unicode License V3; see ../UNICODE_LICENSE.txt.\n",
		unicodeVersion,
		spec.url,
		spec.sha256,
	)
}

func writeGeneratedFile(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".generateunicode-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}
