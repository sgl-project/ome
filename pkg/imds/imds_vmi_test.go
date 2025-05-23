package imds

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sgl-project/sgl-ome/pkg/logging"
)

const (
	vmiInstanceResponse = `{
  "compartmentId": "ocid1.compartment.oc1..aaaaaaaafoobard12345barfoosanitized1234123412431241234213421",
  "regionInfo": {
    "realmDomainComponent": "oraclecloud.com",
    "realmKey": "oc1",
    "regionIdentifier": "us-phoenix-1",
    "regionKey": "PHX"
  }
}`
)

func TestVMI_IMDSResponse(t *testing.T) {
	imdsConfig := DefaultConfig()
	imdsConfig.TimeoutAfter = 10 * time.Second

	client := &fakeHttpDoer{
		instanceResponse: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(vmiInstanceResponse)),
		},
	}

	imdsClient := &Client{
		config:     imdsConfig,
		httpClient: client,
		logger:     logging.NewTestLogger(),
	}

	compartmentID, err := imdsClient.GetCompartmentID()
	require.NoError(t, err)
	require.Equal(t, "ocid1.compartment.oc1..aaaaaaaafoobard12345barfoosanitized1234123412431241234213421", compartmentID)

	realm, err := imdsClient.GetRealm()
	require.NoError(t, err)
	require.Equal(t, "oc1", realm)

	region, err := imdsClient.GetRegion()
	require.NoError(t, err)
	require.Equal(t, "us-phoenix-1", region)
}
