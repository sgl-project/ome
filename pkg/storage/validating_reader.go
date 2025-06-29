package storage

import (
	"io"
)

// ValidatingReader wraps a reader with MD5 validation
type ValidatingReader interface {
	io.ReadCloser
	// Valid returns true if the data read so far is valid
	Valid() bool
	// Expected returns the expected MD5 checksum
	Expected() string
	// Actual returns the actual MD5 checksum calculated so far
	Actual() string
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

// ValidatingWriter wraps a writer with MD5 calculation
type ValidatingWriter interface {
	io.WriteCloser
	// Sum returns the MD5 checksum of data written
	Sum() string
	// SumBase64 returns the MD5 checksum as base64
	SumBase64() string
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
