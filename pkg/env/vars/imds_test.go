package vars

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeIMDSProvider struct {
	realm            string
	region           string
	compartmentId    string
	tenancyId        string
	realmTLD         string
	internalRealmTLD string
	err              error
}

func (f *fakeIMDSProvider) GetRealm() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.realm, nil
}

func (f *fakeIMDSProvider) GetRegion() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.region, nil
}

func (f *fakeIMDSProvider) GetCompartmentID() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.compartmentId, nil
}

func (f *fakeIMDSProvider) GetTenancyID() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.tenancyId, nil
}

func (f *fakeIMDSProvider) GetRealmTLD() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.realmTLD, nil
}

func (f *fakeIMDSProvider) GetInternalRealmTLD() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.internalRealmTLD, nil
}

func TestIMDS_Resolve(t *testing.T) {
	imdsProvider := &fakeIMDSProvider{
		realm:            "region1",
		region:           "r1",
		compartmentId:    "ocid1.compartment.region1..aaaaaaaaprtuilmrj6e4x6zzoanmhj347ssh",
		tenancyId:        "ocid1.tenancy.region1..aaaaaaaaprtuilmrj6e4x6zzoanmhj347ssh",
		internalRealmTLD: "internalrealm.oracleiaas.com",
		realmTLD:         "realm.oracleiaas.com",
		err:              nil,
	}

	r := &IMDSResolver{provider: imdsProvider}

	t.Run("resolve realm", func(t *testing.T) {
		rawRealm, err := r.Resolve(Realm)
		assert.NoError(t, err)
		assert.Equal(t, "region1", rawRealm)
	})
	t.Run("resolve region", func(t *testing.T) {
		rawRegion, err := r.Resolve(Region)
		assert.NoError(t, err)
		assert.Equal(t, "r1", rawRegion)
	})
	t.Run("resolve compartmentID", func(t *testing.T) {
		compID, err := r.Resolve(InstanceCompartmentId)
		assert.NoError(t, err)
		assert.Equal(t, "ocid1-compartment-region1--aaaaaaaaprtuilmrj6e4x6zzoanmhj347ssh", compID)
	})
	t.Run("resolve tenancyID", func(t *testing.T) {
		tenancyID, err := r.Resolve(TenancyId)
		assert.NoError(t, err)
		assert.Equal(t, "ocid1-tenancy-region1--aaaaaaaaprtuilmrj6e4x6zzoanmhj347ssh", tenancyID)
	})
	t.Run("resolve internal realm TLD", func(t *testing.T) {
		internalRealmTLD, err := r.Resolve(InternalRealmTLD)
		assert.NoError(t, err)
		assert.Equal(t, "internalrealm.oracleiaas.com", internalRealmTLD)
	})
	t.Run("resolve internal realm TLD", func(t *testing.T) {
		realmTLD, err := r.Resolve(RealmTLD)
		assert.NoError(t, err)
		assert.Equal(t, "realm.oracleiaas.com", realmTLD)
	})
	t.Run("resolve compartmentID failure", func(t *testing.T) {
		imdsProvider.err = errors.New("expect error")
		_, err := r.Resolve(InstanceCompartmentId)
		assert.Error(t, err)
	})
	t.Run("resolve tenancyID failure", func(t *testing.T) {
		imdsProvider.err = errors.New("expect error")
		_, err := r.Resolve(TenancyId)
		assert.Error(t, err)
	})
	t.Run("resolve region failure", func(t *testing.T) {
		imdsProvider.err = errors.New("expect error")
		_, err := r.Resolve(Region)
		assert.Error(t, err)
	})
	t.Run("resolve some other var failure", func(t *testing.T) {
		imdsProvider.err = errors.New("expect error")
		_, err := r.Resolve(Hostclass)
		assert.Error(t, err)
	})
	t.Run("resolve internal realm TLD failure", func(t *testing.T) {
		imdsProvider.err = errors.New("expect error")
		_, err := r.Resolve(InternalRealmTLD)
		assert.Error(t, err)
	})
	t.Run("resolve realm TLD failure", func(t *testing.T) {
		imdsProvider.err = errors.New("expect error")
		_, err := r.Resolve(RealmTLD)
		assert.Error(t, err)
	})
}
