package modelagent

func sameStringPtr(left *string, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func sameStringMapPtr(left *map[string]string, right *map[string]string) bool {
	if left == nil || right == nil {
		return left == right
	}
	if len(*left) != len(*right) {
		return false
	}
	for key, leftValue := range *left {
		if rightValue, ok := (*right)[key]; !ok || rightValue != leftValue {
			return false
		}
	}
	return true
}
