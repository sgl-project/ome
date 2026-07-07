package components

import (
	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/utils/storage"
)

// isPVCBaseModel reports whether b's referenced BaseModel is PVC-backed.
func isPVCBaseModel(b *BaseComponentFields) bool {
	if b == nil || b.BaseModel == nil {
		return false
	}
	return isPVCStorage(b.BaseModel.Storage)
}

// parsePVCComponents returns the parsed PVC URI for b's BaseModel. Returns
// nil if the model is not PVC-backed or the URI is malformed; the caller is
// expected to have validated the model upstream (via the BaseModel
// reconciler / admission webhook), so a parse error here is treated as
// "no PVC" rather than fatal.
func parsePVCComponents(b *BaseComponentFields) *storage.PVCStorageComponents {
	if !isPVCBaseModel(b) {
		return nil
	}
	return parsePVCStorage(b.BaseModel.Storage)
}

// isPVCStorage / parsePVCStorage are the StorageSpec-level forms behind the
// BaseModel helpers above.
func isPVCStorage(s *v1beta1.StorageSpec) bool {
	if s == nil || s.StorageUri == nil {
		return false
	}
	t, err := storage.GetStorageType(*s.StorageUri)
	if err != nil {
		return false
	}
	return t == storage.StorageTypePVC
}

func parsePVCStorage(s *v1beta1.StorageSpec) *storage.PVCStorageComponents {
	if !isPVCStorage(s) {
		return nil
	}
	components, err := storage.ParsePVCStorageURI(*s.StorageUri)
	if err != nil {
		return nil
	}
	return components
}
