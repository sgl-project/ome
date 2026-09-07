package printers

// Unicode property tables and the grapheme conformance fixture are generated
// from Unicode 17.0.0 sources whose URLs and SHA-256 digests are pinned in
// internal/generateunicode. The generator verifies every source before writing
// either output. Tests use only committed data and never access the network.
//
// To regenerate from the pinned upstream files, run `go generate` in this
// directory. For an offline reproduction, invoke the same generator with its
// grapheme-property, emoji-data, derived-core-properties, east-asian-width,
// and grapheme-break-test `-*-source` flags pointing at previously downloaded,
// byte-identical source files.
//
//go:generate go run ./internal/generateunicode
