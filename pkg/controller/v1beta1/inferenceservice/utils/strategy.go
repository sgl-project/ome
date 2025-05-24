package utils

import (
	v1beta1api "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
)

// MemoryStrategy TODO MemoryStrategy will be implemented in another PR
type MemoryStrategy struct {
}

func (v *MemoryStrategy) GetShard(isvc *v1beta1api.InferenceService) []int {
	// TODO to be implemented in another PR
	// Currently each InferenceService only has one shard with id=0
	return []int{0}
}
