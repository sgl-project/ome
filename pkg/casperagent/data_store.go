package casper

type ObjectURI struct {
	Namespace  string `mapstructure:"namespace" json:"namespace"`
	BucketName string `mapstructure:"bucket_name" json:"bucket_name"`
	ObjectName string `mapstructure:"object_name" json:"object_name"`
	Prefix     string `mapstructure:"prefix" json:"prefix"`
	IsVendor   bool   `mapstructure:"is_vendor" json:"is_vendor"`
}

type DataStore interface {
	// Download downloads an object from its source path to the target path.
	Download(source ObjectURI, target string) error

	// Upload uploads an object from its source path to the target path
	Upload(source string, target ObjectURI) error
}
