package oci

import (
	"io"
	"strings"
)

// IsReaderEmpty checks if a reader is empty without consuming it
func IsReaderEmpty(streamReader io.Reader) bool {
	switch v := streamReader.(type) {
	case *strings.Reader:
		return v.Len() == 0
	case interface{ Len() int }:
		return v.Len() == 0
	default:
		return false
	}
}

// RemoveOpcMetaPrefix removes "opc-meta-" prefix from metadata keys
func RemoveOpcMetaPrefix(metadata map[string]string) map[string]string {
	if metadata == nil {
		return metadata
	}

	updatedMetadata := make(map[string]string)
	for key, value := range metadata {
		if strings.HasPrefix(key, "opc-meta-") {
			// Remove "opc-meta-" prefix
			newKey := key[9:]
			updatedMetadata[newKey] = value
		} else {
			// Keep original key-value pair
			updatedMetadata[key] = value
		}
	}
	return updatedMetadata
}
