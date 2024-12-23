package imds

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sampleInstanceResultJSONPayload = `{
  "compartmentId": "ocid1.compartment.oc5..aaaaaaaasnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnip",
  "definedTags": {
    "OracleInternalReserved": {
      "UsageType": "development-qa-test"
    },
    "host-access": {
      "multiple-values-test": "prod",
      "subclass": "my-subclass"
    }
  },
  "regionInfo": {
    "realmKey": "oc5",
    "regionIdentifier": "us-tacoma-1"
  },
  "metadata": {
    "hostclass": "permissions-service",
    "hostsubclass": "my-legacy-subclass"
  }
}`
)

func TestInstanceResult(t *testing.T) {
	t.Run("parsing all values", func(t *testing.T) {
		actual := unmarshalSampleInstancePayload(t)

		assert.Equal(t, actual.RegionInfo.Region, "us-tacoma-1")
		assert.Equal(t, actual.RegionInfo.Realm, "oc5")
		assert.Equal(t, actual.Metadata.Hostclass, "permissions-service")
		assert.Equal(t, actual.Metadata.Hostsubclass, "my-legacy-subclass")
		assert.Equal(t, actual.CompartmentId, "ocid1.compartment.oc5..aaaaaaaasnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnip")
		assert.Equal(t, actual.DefinedTags, DefinedTags{
			"OracleInternalReserved": {
				"UsageType": "development-qa-test",
			},
			"host-access": {
				"multiple-values-test": "prod",
				"subclass":             "my-subclass",
			},
		})
	})

	t.Run("toInstanceResult", func(t *testing.T) {
		payloadJSON := unmarshalSampleInstancePayload(t)

		actual, err := payloadJSON.asInstanceResult()
		require.NoError(t, err)

		require.Equal(t, instanceResult{
			Realm:         "oc5",
			Region:        "us-tacoma-1",
			CompartmentId: "ocid1.compartment.oc5..aaaaaaaasnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnip",
			Hostclass:     "permissions-service",
			HostSubclass:  "my-subclass", // defined tags win in case of both being present
		}, actual)
	})

	t.Run("toInstanceResult no defined tags", func(t *testing.T) {
		payloadJSON := unmarshalSampleInstancePayload(t)
		payloadJSON.DefinedTags = nil

		actual, err := payloadJSON.asInstanceResult()
		require.NoError(t, err)

		require.Equal(t, instanceResult{
			Realm:         "oc5",
			Region:        "us-tacoma-1",
			CompartmentId: "ocid1.compartment.oc5..aaaaaaaasnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnip",
			Hostclass:     "permissions-service",
			HostSubclass:  "my-legacy-subclass", // fall back onto metadata.hostsubclass
		}, actual)
	})

	t.Run("toInstanceResult no defined host-access.subclass tag", func(t *testing.T) {
		payloadJSON := unmarshalSampleInstancePayload(t)
		delete(payloadJSON.DefinedTags, "host-access")

		actual, err := payloadJSON.asInstanceResult()
		require.NoError(t, err)

		require.Equal(t, instanceResult{
			Realm:         "oc5",
			Region:        "us-tacoma-1",
			CompartmentId: "ocid1.compartment.oc5..aaaaaaaasnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnip",
			Hostclass:     "permissions-service",
			HostSubclass:  "my-legacy-subclass", // fall back onto metadata.hostsubclass
		}, actual)
	})

	t.Run("toInstanceResult no subclasses defined", func(t *testing.T) {
		payloadJSON := unmarshalSampleInstancePayload(t)
		delete(payloadJSON.DefinedTags, "host-access")
		payloadJSON.Metadata.Hostsubclass = ""

		actual, err := payloadJSON.asInstanceResult()
		require.NoError(t, err)

		require.Equal(t, instanceResult{
			Realm:         "oc5",
			Region:        "us-tacoma-1",
			CompartmentId: "ocid1.compartment.oc5..aaaaaaaasnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnip",
			Hostclass:     "permissions-service",
			HostSubclass:  "",
		}, actual)
	})

	t.Run("toInstanceResult no metadata.hostsubclass defined", func(t *testing.T) {
		payloadJSON := unmarshalSampleInstancePayload(t)
		payloadJSON.Metadata.Hostsubclass = ""

		actual, err := payloadJSON.asInstanceResult()
		require.NoError(t, err)

		require.Equal(t, instanceResult{
			Realm:         "oc5",
			Region:        "us-tacoma-1",
			CompartmentId: "ocid1.compartment.oc5..aaaaaaaasnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnipsnip",
			Hostclass:     "permissions-service",
			HostSubclass:  "my-subclass",
		}, actual)
	})
}

func unmarshalSampleInstancePayload(t *testing.T) instanceResultRaw {
	var actual instanceResultRaw
	require.NoError(t, json.Unmarshal([]byte(sampleInstanceResultJSONPayload), &actual))
	return actual
}
