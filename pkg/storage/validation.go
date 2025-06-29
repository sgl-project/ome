package storage

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// MD5Validator provides MD5 checksum validation functionality
type MD5Validator struct {
	hash     io.Writer
	checksum string
}

// NewMD5Validator creates a new MD5 validator
func NewMD5Validator() *MD5Validator {
	return &MD5Validator{
		hash: md5.New(),
	}
}

// Write implements io.Writer interface for streaming MD5 calculation
func (v *MD5Validator) Write(p []byte) (n int, err error) {
	return v.hash.Write(p)
}

// Sum returns the calculated MD5 checksum as a hex string
func (v *MD5Validator) Sum() string {
	return hex.EncodeToString(v.hash.(interface{ Sum([]byte) []byte }).Sum(nil))
}

// SumBase64 returns the calculated MD5 checksum as a base64 string
func (v *MD5Validator) SumBase64() string {
	return base64.StdEncoding.EncodeToString(v.hash.(interface{ Sum([]byte) []byte }).Sum(nil))
}

// Validate checks if the calculated checksum matches the expected value
func (v *MD5Validator) Validate(expected string) bool {
	if expected == "" {
		return true // No validation if expected is empty
	}

	calculated := v.Sum()

	// Try different formats
	if calculated == expected {
		return true
	}

	// Try base64 format
	if v.SumBase64() == expected {
		return true
	}

	// Try with different case
	if strings.EqualFold(calculated, expected) {
		return true
	}

	// Try decoding base64 and comparing
	if decoded, err := base64.StdEncoding.DecodeString(expected); err == nil {
		if calculated == hex.EncodeToString(decoded) {
			return true
		}
	}

	return false
}

// ValidateFileMD5 validates a file's MD5 checksum
func ValidateFileMD5(filepath string, expectedMD5 string) (bool, error) {
	if expectedMD5 == "" {
		return true, nil // No validation needed
	}

	file, err := os.Open(filepath)
	if err != nil {
		return false, fmt.Errorf("failed to open file for MD5 validation: %w", err)
	}
	defer file.Close()

	validator := NewMD5Validator()
	if _, err := io.Copy(validator, file); err != nil {
		return false, fmt.Errorf("failed to calculate MD5: %w", err)
	}

	return validator.Validate(expectedMD5), nil
}

// ValidateReaderMD5 validates data from a reader against expected MD5
func ValidateReaderMD5(reader io.Reader, expectedMD5 string) (io.Reader, *MD5Validator, error) {
	if expectedMD5 == "" {
		// No validation needed, return reader as-is
		return reader, nil, nil
	}

	validator := NewMD5Validator()
	// Create a tee reader that writes to both the validator and passes through
	teeReader := io.TeeReader(reader, validator)

	return teeReader, validator, nil
}

// MultipartMD5 handles MD5 validation for multipart uploads/downloads
type MultipartMD5 struct {
	parts      []string
	partHashes []string
}

// NewMultipartMD5 creates a new multipart MD5 validator
func NewMultipartMD5() *MultipartMD5 {
	return &MultipartMD5{
		parts:      make([]string, 0),
		partHashes: make([]string, 0),
	}
}

// AddPart adds a part's MD5 to the multipart validator
func (m *MultipartMD5) AddPart(partMD5 string) {
	m.parts = append(m.parts, partMD5)
}

// AddPartData calculates and adds MD5 for part data
func (m *MultipartMD5) AddPartData(data []byte) {
	hash := md5.Sum(data)
	m.parts = append(m.parts, hex.EncodeToString(hash[:]))
}

// ComputeMultipartMD5 computes the final multipart MD5
// This follows the S3 multipart upload MD5 calculation:
// MD5(MD5(part1) + MD5(part2) + ... + MD5(partN))-N
func (m *MultipartMD5) ComputeMultipartMD5() string {
	if len(m.parts) == 0 {
		return ""
	}

	if len(m.parts) == 1 {
		return m.parts[0] // Single part, return as-is
	}

	// Concatenate all part MD5s
	concatenated := make([]byte, 0, len(m.parts)*16)
	for _, partMD5 := range m.parts {
		decoded, err := hex.DecodeString(partMD5)
		if err != nil {
			continue // Skip invalid MD5s
		}
		concatenated = append(concatenated, decoded...)
	}

	// Calculate MD5 of concatenated MD5s
	finalHash := md5.Sum(concatenated)

	// Return in S3 multipart format: hash-numparts
	return fmt.Sprintf("%s-%d", hex.EncodeToString(finalHash[:]), len(m.parts))
}

// IsMultipartMD5 checks if an MD5 string is in multipart format
func IsMultipartMD5(md5String string) bool {
	// Multipart MD5s have the format: hash-numparts
	parts := strings.Split(md5String, "-")
	if len(parts) != 2 {
		return false
	}

	// Check if the hash part is valid hex
	if _, err := hex.DecodeString(parts[0]); err != nil {
		return false
	}

	// Check if the number part is valid
	if _, err := fmt.Sscanf(parts[1], "%d", new(int)); err != nil {
		return false
	}

	return true
}

// validatingReader implements ValidatingReader
type validatingReader struct {
	reader    io.ReadCloser
	validator *MD5Validator
	expected  string
	validated bool
}

// NewValidatingReader creates a new validating reader
func NewValidatingReader(reader io.ReadCloser, expectedMD5 string) ValidatingReader {
	return &validatingReader{
		reader:    reader,
		validator: NewMD5Validator(),
		expected:  expectedMD5,
	}
}

// Read implements io.Reader
func (vr *validatingReader) Read(p []byte) (n int, err error) {
	n, err = vr.reader.Read(p)
	if n > 0 {
		vr.validator.Write(p[:n])
	}

	// If we've reached EOF, validate
	if err == io.EOF && vr.expected != "" {
		vr.validated = vr.validator.Validate(vr.expected)
	}

	return n, err
}

// Close implements io.Closer
func (vr *validatingReader) Close() error {
	return vr.reader.Close()
}

// Valid returns true if the data is valid
func (vr *validatingReader) Valid() bool {
	if vr.expected == "" {
		return true // No validation required
	}
	return vr.validated
}

// Expected returns the expected MD5
func (vr *validatingReader) Expected() string {
	return vr.expected
}

// Actual returns the calculated MD5
func (vr *validatingReader) Actual() string {
	return vr.validator.Sum()
}

// validatingWriter implements ValidatingWriter
type validatingWriter struct {
	writer    io.WriteCloser
	validator *MD5Validator
}

// NewValidatingWriter creates a new validating writer
func NewValidatingWriter(writer io.WriteCloser) ValidatingWriter {
	return &validatingWriter{
		writer:    writer,
		validator: NewMD5Validator(),
	}
}

// Write implements io.Writer
func (vw *validatingWriter) Write(p []byte) (n int, err error) {
	n, err = vw.writer.Write(p)
	if n > 0 {
		vw.validator.Write(p[:n])
	}
	return n, err
}

// Close implements io.Closer
func (vw *validatingWriter) Close() error {
	return vw.writer.Close()
}

// Sum returns the MD5 checksum
func (vw *validatingWriter) Sum() string {
	return vw.validator.Sum()
}

// SumBase64 returns the MD5 checksum as base64
func (vw *validatingWriter) SumBase64() string {
	return vw.validator.SumBase64()
}

// TeeValidatingReader creates a reader that validates while passing data through
func TeeValidatingReader(reader io.Reader, expectedMD5 string) (io.Reader, *MD5Validator) {
	validator := NewMD5Validator()
	teeReader := io.TeeReader(reader, validator)
	return teeReader, validator
}
