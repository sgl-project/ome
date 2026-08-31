package irstatus

import "errors"

const genericCodecErrorMessage = "instance status codec failed"

// ErrorReason is a bounded classification for status codec failures.
type ErrorReason string

const (
	ErrorReasonUnknownEncoding     ErrorReason = "unknown_encoding"
	ErrorReasonRepresentationUnion ErrorReason = "representation_union"
	ErrorReasonRangeSyntax         ErrorReason = "range_syntax"
	ErrorReasonRangeOrder          ErrorReason = "range_order"
	ErrorReasonRangeOverflow       ErrorReason = "range_overflow"
	ErrorReasonCardinalityLimit    ErrorReason = "cardinality_limit"
	ErrorReasonCoverage            ErrorReason = "coverage"
	ErrorReasonCanonicalOrder      ErrorReason = "canonical_order"
	ErrorReasonValueDomain         ErrorReason = "value_domain"
)

// codecError reports a fixed-catalog failure without including status data.
type codecError struct {
	reason ErrorReason
}

func (e *codecError) Error() string {
	if e == nil {
		return genericCodecErrorMessage
	}
	return e.reason.Message()
}

// Message returns the fixed operator-facing text for one codec failure class.
func (r ErrorReason) Message() string {
	message, _ := r.catalogMessage()
	return message
}

func (r ErrorReason) catalogMessage() (string, bool) {
	switch r {
	case ErrorReasonUnknownEncoding:
		return "instance status encoding is unknown", true
	case ErrorReasonRepresentationUnion:
		return "instance status representation is invalid", true
	case ErrorReasonRangeSyntax:
		return "instance index set syntax is invalid", true
	case ErrorReasonRangeOrder:
		return "instance index set order is invalid", true
	case ErrorReasonRangeOverflow:
		return "instance index set exceeds the supported index domain", true
	case ErrorReasonCardinalityLimit:
		return "instance status cardinality limit is invalid", true
	case ErrorReasonCoverage:
		return "instance status column coverage is invalid", true
	case ErrorReasonCanonicalOrder:
		return "instance status column order is not canonical", true
	case ErrorReasonValueDomain:
		return "instance status value is outside its supported domain", true
	default:
		return genericCodecErrorMessage, false
	}
}

// ErrorReasonOf extracts a codec failure classification through wrapping.
func ErrorReasonOf(err error) (ErrorReason, bool) {
	var codecErr *codecError
	if !errors.As(err, &codecErr) || codecErr == nil {
		return "", false
	}
	if _, known := codecErr.reason.catalogMessage(); !known {
		return "", false
	}
	return codecErr.reason, true
}

func newCodecError(reason ErrorReason) error {
	return &codecError{reason: reason}
}
