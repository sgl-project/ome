package vibe

import (
	"encoding/json"
	"errors"

	"github.com/spf13/afero"
)

// Client holds vibe Metadata.
type Client struct {
	metadata Metadata
}

// NewMetadataClient creates a new vibe.Client.
func NewMetadataClient(config Config, fs afero.Fs) (Client, error) {
	if fs == nil {
		return Client{}, errors.New("nil filesystem")
	}

	// load the content of the file
	resultBytes, err := afero.ReadFile(fs, config.MetadataFilePath)
	if err != nil {
		return Client{}, err
	}

	var metadata Metadata
	err = json.Unmarshal(resultBytes, &metadata)
	if err != nil {
		return Client{}, err
	}

	return Client{
		metadata: metadata,
	}, nil
}

// GetRealm returns vibe target realm.
func (c Client) GetRealm() string {
	return c.metadata.Realm
}

// GetRegion returns vibe target region.
func (c Client) GetRegion() string {
	return c.metadata.Region
}

// GetAirportCode returns vibe target region airport code.
func (c Client) GetAirportCode() string {
	return c.metadata.Airport
}

// GetSvcEnclaveSuffix returns service enclave DNS suffix.
func (c Client) GetSvcEnclaveSuffix() string {
	return c.metadata.ServiceEnclaveDNSSuffix
}

// GetIdentityRealm returns vibe identity realm.
func (c Client) GetIdentityRealm() string {
	return c.metadata.IdentityRealm
}

// GetLdapifiedRegion returns region names recognized by LDAP entitlements.
func (c Client) GetLdapifiedRegion() string {
	switch c.metadata.Region {
	case "sea", "us-seattle-1", "r1":
		return "r1"
	case "phx", "us-phoenix-1", "r2":
		return "r2"
	default:
		return c.metadata.Region
	}
}

// GetLdapifiedRealm returns realm names recognized by LDAP entitlements.
func (c Client) GetLdapifiedRealm() string {
	switch c.metadata.Realm {
	case "oc1":
		return "prod"
	case "region1":
		return "r1"
	default:
		return c.metadata.Realm
	}
}
