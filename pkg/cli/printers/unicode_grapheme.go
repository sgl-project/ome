package printers

import "unicode"

// segmentDisplayClusters implements the extended grapheme-cluster rules in
// Unicode Standard Annex #29, revision 47 (Unicode 17.0.0). Width is computed
// from terminal cells after segmentation so a hard wrap never splits a valid
// cluster.
func segmentDisplayClusters(value string) []displayCluster {
	runes := []rune(value)
	if len(runes) == 0 {
		return nil
	}

	properties := make([]graphemeProperty, len(runes))
	indicProperties := make([]indicProperty, len(runes))
	extendedPictographic := make([]bool, len(runes))
	for i, char := range runes {
		properties[i] = lookupGraphemeProperty(char)
		indicProperties[i] = lookupIndicProperty(char)
		extendedPictographic[i] = runeInIntervals(char, extendedPictographicRanges)
	}
	clusters := make([]displayCluster, 0, len(runes))
	start := 0
	regionalIndicatorOdd := false
	for boundary := 1; boundary <= len(runes); boundary++ {
		regionalIndicatorOdd = nextRegionalIndicatorOddParity(
			regionalIndicatorOdd, properties[boundary-1],
		)
		if boundary < len(runes) && !breaksGrapheme(
			properties, indicProperties, extendedPictographic, regionalIndicatorOdd, boundary,
		) {
			continue
		}
		clusters = append(clusters, displayCluster{
			value: string(runes[start:boundary]),
			width: graphemeDisplayWidth(
				runes[start:boundary], properties[start:boundary], extendedPictographic[start:boundary],
			),
		})
		start = boundary
	}
	return clusters
}

func breaksGrapheme(
	properties []graphemeProperty,
	indicProperties []indicProperty,
	extendedPictographic []bool,
	regionalIndicatorOdd bool,
	boundary int,
) bool {
	left := properties[boundary-1]
	right := properties[boundary]

	// GB3: CR × LF.
	if left == graphemeCR && right == graphemeLF {
		return false
	}
	// GB4 and GB5: break around controls.
	if isGraphemeControl(left) || isGraphemeControl(right) {
		return true
	}
	// GB6 through GB8: Hangul syllable sequences.
	if left == graphemeL && (right == graphemeL || right == graphemeV || right == graphemeLV || right == graphemeLVT) {
		return false
	}
	if (left == graphemeLV || left == graphemeV) && (right == graphemeV || right == graphemeT) {
		return false
	}
	if (left == graphemeLVT || left == graphemeT) && right == graphemeT {
		return false
	}
	// GB9 and GB9a: extending characters stay with the preceding cluster.
	if right == graphemeExtend || right == graphemeZWJ || right == graphemeSpacingMark {
		return false
	}
	// GB9b: prepending characters stay with the following cluster.
	if left == graphemePrepend {
		return false
	}
	// GB9c: keep an Indic consonant-linker-consonant sequence together.
	if indicProperties[boundary] == indicConsonant && hasIndicLinkerPrefix(indicProperties, boundary) {
		return false
	}
	// GB11: an extended-pictographic ZWJ sequence is one cluster.
	if extendedPictographic[boundary] && hasExtendedPictographicZWJPrefix(properties, extendedPictographic, boundary) {
		return false
	}
	// GB12 and GB13: pair regional indicators from the start of their run.
	if left == graphemeRegionalIndicator && right == graphemeRegionalIndicator {
		return !regionalIndicatorOdd
	}

	// GB999: break everywhere else.
	return true
}

func nextRegionalIndicatorOddParity(previous bool, property graphemeProperty) bool {
	if property == graphemeRegionalIndicator {
		return !previous
	}
	return false
}

func isGraphemeControl(property graphemeProperty) bool {
	return property == graphemeCR || property == graphemeLF || property == graphemeControl
}

func hasIndicLinkerPrefix(properties []indicProperty, boundary int) bool {
	foundLinker := false
	for i := boundary - 1; i >= 0; i-- {
		switch properties[i] {
		case indicExtend:
			continue
		case indicLinker:
			foundLinker = true
			continue
		case indicConsonant:
			return foundLinker
		default:
			return false
		}
	}
	return false
}

func hasExtendedPictographicZWJPrefix(
	properties []graphemeProperty,
	extendedPictographic []bool,
	boundary int,
) bool {
	index := boundary - 1
	if index < 0 || properties[index] != graphemeZWJ {
		return false
	}
	index--
	for index >= 0 && properties[index] == graphemeExtend {
		index--
	}
	return index >= 0 && extendedPictographic[index]
}

func graphemeDisplayWidth(
	runes []rune,
	properties []graphemeProperty,
	extendedPictographic []bool,
) int {
	if len(runes) == 0 {
		return 0
	}
	prefixEnd := 0
	prefixWidth := 0
	for prefixEnd < len(runes) && properties[prefixEnd] == graphemePrepend {
		prefixWidth += terminalRuneWidth(runes[prefixEnd], properties[prefixEnd])
		prefixEnd++
	}
	if prefixEnd == len(runes) {
		return prefixWidth
	}
	return prefixWidth + graphemeCoreDisplayWidth(
		runes[prefixEnd:], properties[prefixEnd:], extendedPictographic[prefixEnd:],
	)
}

func graphemeCoreDisplayWidth(
	runes []rune,
	properties []graphemeProperty,
	extendedPictographic []bool,
) int {
	if end := keycapSequenceEnd(runes); end > 0 {
		return 2 + trailingDisplayWidth(runes[end:], properties[end:])
	}
	if properties[0] == graphemeRegionalIndicator {
		end := 1
		if len(properties) > 1 && properties[1] == graphemeRegionalIndicator {
			end = 2
		}
		return 2 + trailingDisplayWidth(runes[end:], properties[end:])
	}
	if properties[0] == graphemeL && len(runes) > 1 {
		// Decomposed Hangul Jamo compose into one two-cell syllable.
		end := 1
		for end < len(properties) && isHangulProperty(properties[end]) {
			end++
		}
		return 2 + trailingDisplayWidth(runes[end:], properties[end:])
	}
	if extendedPictographic[0] {
		end, width := emojiCoreWidth(runes, properties, extendedPictographic)
		return width + trailingDisplayWidth(runes[end:], properties[end:])
	}
	return trailingDisplayWidth(runes, properties)
}

func terminalRuneWidth(char rune, property graphemeProperty) int {
	// Emoji modifiers have Grapheme_Cluster_Break=Extend, but they still occupy
	// two cells when they are not consumed by an Emoji_Modifier_Base sequence.
	if isEmojiModifier(char) {
		return 2
	}
	// Format controls influence adjacent text without occupying a terminal
	// cell. Some of them have Grapheme_Cluster_Break=Prepend rather than Extend.
	if unicode.Is(unicode.Cf, char) {
		return 0
	}
	if property == graphemeExtend && unicode.Is(unicode.Mc, char) && isWideRune(char) {
		return 2
	}
	if property == graphemeControl || property == graphemeCR || property == graphemeLF ||
		property == graphemeExtend || property == graphemeZWJ {
		return 0
	}
	if isWideRune(char) {
		return 2
	}
	return 1
}

func trailingDisplayWidth(runes []rune, properties []graphemeProperty) int {
	width := 0
	for i, char := range runes {
		width += terminalRuneWidth(char, properties[i])
	}
	return width
}

func isHangulProperty(property graphemeProperty) bool {
	return property == graphemeL || property == graphemeV || property == graphemeT ||
		property == graphemeLV || property == graphemeLVT
}

func keycapSequenceEnd(runes []rune) int {
	if len(runes) < 2 || !isKeycapBase(runes[0]) {
		return 0
	}
	index := 1
	if index < len(runes) && runes[index] == 0xfe0f {
		index++
	}
	if index < len(runes) && runes[index] == 0x20e3 {
		return index + 1
	}
	return 0
}

func isKeycapBase(char rune) bool {
	return char == '#' || char == '*' || (char >= '0' && char <= '9')
}

func emojiCoreWidth(
	runes []rune,
	properties []graphemeProperty,
	extendedPictographic []bool,
) (int, int) {
	end, emojiPresentation, textPresentation := emojiBaseSuffixEnd(runes, 0)
	emojiSequence := emojiPresentation
	if end > 1 && isEmojiModifier(runes[end-1]) {
		emojiSequence = true
	}
	preservedExtendWidth := 0

	for {
		joiner := end
		for joiner < len(runes) && properties[joiner] == graphemeExtend {
			joiner++
		}
		if joiner+1 >= len(runes) || properties[joiner] != graphemeZWJ ||
			!extendedPictographic[joiner+1] {
			break
		}
		emojiSequence = true
		preservedExtendWidth += trailingDisplayWidth(runes[end:joiner], properties[end:joiner])
		end, _, _ = emojiBaseSuffixEnd(runes, joiner+1)
	}

	if emojiSequence {
		return end, 2 + preservedExtendWidth
	}
	if textPresentation {
		return end, 1
	}
	return end, terminalRuneWidth(runes[0], properties[0])
}

func emojiBaseSuffixEnd(runes []rune, base int) (int, bool, bool) {
	end := base + 1
	emojiPresentation := false
	textPresentation := false
	if end < len(runes) {
		switch runes[end] {
		case 0xfe0e:
			textPresentation = true
			end++
		case 0xfe0f:
			emojiPresentation = true
			end++
		}
	}
	if end < len(runes) && isEmojiModifier(runes[end]) &&
		runeInIntervals(runes[base], emojiModifierBaseRanges) {
		end++
	}
	return end, emojiPresentation, textPresentation
}

func lookupGraphemeProperty(char rune) graphemeProperty {
	low, high := 0, len(graphemePropertyRanges)
	for low < high {
		middle := low + (high-low)/2
		interval := graphemePropertyRanges[middle]
		if char < interval.first {
			high = middle
			continue
		}
		if char > interval.last {
			low = middle + 1
			continue
		}
		return interval.property
	}
	return graphemeOther
}

func lookupIndicProperty(char rune) indicProperty {
	low, high := 0, len(indicPropertyRanges)
	for low < high {
		middle := low + (high-low)/2
		interval := indicPropertyRanges[middle]
		if char < interval.first {
			high = middle
			continue
		}
		if char > interval.last {
			low = middle + 1
			continue
		}
		return interval.property
	}
	return indicNone
}

func runeInIntervals(char rune, intervals []runeInterval) bool {
	low, high := 0, len(intervals)
	for low < high {
		middle := low + (high-low)/2
		interval := intervals[middle]
		if char < interval.first {
			high = middle
			continue
		}
		if char > interval.last {
			low = middle + 1
			continue
		}
		return true
	}
	return false
}
