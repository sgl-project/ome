package casper

type ObjectURI struct {
	Namespace  string `mapstructure:"namespace" json:"namespace"`
	BucketName string `mapstructure:"bucket_name" json:"bucket_name" validate:"required"`
	ObjectName string `mapstructure:"object_name" json:"object_name"`
	Prefix     string `mapstructure:"prefix" json:"prefix"`
}

type DataStore interface {
	// Download downloads an object from its source path to the target path.
	Download(source ObjectURI, target string) error

	// Upload uploads an object from its source path to the target path
	Upload(source string, target ObjectURI) error
}
