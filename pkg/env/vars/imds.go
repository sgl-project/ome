package vars

import (
	"fmt"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/env/imds"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/logging"
)

var (
	InstanceCompartmentId = MustNewVar("instanceCompartmentID", true)
	TenancyId             = MustNewVar("tenancyID", true)
)

type imdsProvider interface {
	GetRealm() (string, error)
	GetRegion() (string, error)
	GetCompartmentID() (string, error)
	GetTenancyID() (string, error)
	GetRealmTLD() (string, error)
	GetInternalRealmTLD() (string, error)
}

// IMDSResolver represents a instance-metadata var resolver
type IMDSResolver struct {
	provider imdsProvider
}

func NewIMDSResolver(config imds.Config, logger logging.Interface) (*IMDSResolver, error) {
	provider, err := imds.NewClient(config, logger)
	if err != nil {
		return nil, fmt.Errorf("constructing imds provider: %w", err)
	}

	return &IMDSResolver{provider: provider}, nil
}

func (o IMDSResolver) CanResolve() []Var {
	return []Var{
		Realm,
		Region,
		InstanceCompartmentId,
		TenancyId,
		RealmTLD,
		InternalRealmTLD,
	}
}

func (o IMDSResolver) Resolve(v Var) (string, error) {
	switch v {
	case Realm:
		return o.provider.GetRealm()
	case Region:
		return o.provider.GetRegion()
	case InstanceCompartmentId:
		ocid, err := o.provider.GetCompartmentID()
		if err != nil {
			return "", err
		}

		return escapeOCID(ocid), nil
	case TenancyId:
		ocid, err := o.provider.GetTenancyID()
		if err != nil {
			return "", err
		}

		return escapeOCID(ocid), nil
	case RealmTLD:
		return o.provider.GetRealmTLD()
	case InternalRealmTLD:
		return o.provider.GetInternalRealmTLD()
	}

	return "", fmt.Errorf("IMDSResolver can't resolve var: %v", v)
}

var _ Resolver = &IMDSResolver{}
