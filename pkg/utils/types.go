package utils

func Bool(b bool) *bool {
	return &b
}

func UInt64(u uint64) *uint64 {
	return &u
}

// PtrStrEqual reports whether two *string carry equal values. Two nil
// pointers are equal; a nil and a non-nil are not (even when the
// non-nil points at "").
func PtrStrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
