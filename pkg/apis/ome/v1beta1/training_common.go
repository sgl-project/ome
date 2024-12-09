package v1beta1

// StorageSource represents the different types of storage
type StorageSource string

const (
	ObjectStorage StorageSource = "OBJECT_STORAGE"
	PVC           StorageSource = "PVC"
)

// Storage defines the storage source/location of the data/model
// The fields follow a "1-of" semantic. Users must specify exactly one URL based on given source.
type Storage struct {
	// Represents the type of the storage
	StorageType StorageSource `json:"storageType,omitempty"`

	// ObjectStorageSpec for data/model stored in ObjectStorage
	OSStorageSpec *OSStorage `json:"oSStorageSpec,omitempty"`

	// PVCStorageSpec for data/model stored as PVC
	PVCStorageSpec *PVCStorage `json:"pVCStorageSpec,omitempty"`
}

// OSStorage defines the arguments for object storage
type OSStorage struct {
	BucketName string `json:"bucketName"`
	Namespace  string `json:"namespace"`
	ObjectName string `json:"objectName,omitempty"`
	Prefix     string `json:"prefix,omitempty"`
	OboToken   string `json:"oboToken,omitempty"`
}

// PVCStorage defines the arguments for pvc storage
type PVCStorage struct {
	// This field points to the location of the data/model which is mounted onto the pod.
	Path string `json:"path"`
}
