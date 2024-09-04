package model_agent

import (
	"fmt"
	"strings"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casperagent"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principalsagent"
)

func NewCasperDataStore(authType string) (casper.CasperDataStore, error) {
	var authProvider principals.AuthProvider
	switch authType {
	case "workload-identity":
		authProvider = &principals.WorkloadIdentityAuthProvider{}
	default:
		authProvider = &principals.InstancePrincipalAuthProvider{}
	}

	configProvider, err := authProvider.ConfigurationProvider()
	if err != nil {
		return casper.CasperDataStore{}, err
	}

	casperClient, err := casper.GetObjectStorageClient(configProvider, "")
	if err != nil {
		return casper.CasperDataStore{}, fmt.Errorf("failed to create ObjectStorageClient for source tenancy: %+v", err)
	}

	return casper.CasperDataStore{
		CasperClient: casperClient,
	}, nil
}

/*
Sample:
"os://n/idqj093njucb/b/bucketName/o/model-name-dir/model-object"
idqj093njucb index0
bucketName index1
objectPath path after index1
*/
func NewObjectStorageUri(storageUrl string) (*casper.ObjectURI, error) {
	if err := validateStorageUrl(storageUrl); err != nil {
		return nil, err
	}

	objectStorageUrl := strings.Split(storageUrl, constants.ObjectStorageUrlPrefix)[1]
	values := strings.Split(objectStorageUrl, "/")
	objectPath := strings.Join(values[5:], "/")

	osInfo := &casper.ObjectURI{
		Namespace:  values[1],
		BucketName: values[3],
		Prefix:     objectPath,
	}

	return osInfo, nil
}

func validateStorageUrl(storageUrl string) error {
	errMsg := "invalid object storage uri. should be in `#  oci://n/${namespace}/b/${bucket}/o/${object-path}` format"
	if !strings.HasPrefix(storageUrl, constants.ObjectStorageUrlPrefix) {
		return fmt.Errorf(errMsg)
	}

	objectStorageUrl := strings.Split(storageUrl, constants.ObjectStorageUrlPrefix)[1]
	values := strings.Split(objectStorageUrl, "/")
	if len(values) < 6 {
		return fmt.Errorf(errMsg)
	}

	return nil
}
