package imds

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sgl-project/sgl-ome/pkg/logging"
)

const (
	sampleInstanceCert = `-----BEGIN CERTIFICATE-----
MIIIAjCCBeqgAwIBAgIRAO9NdRPN+EQw2Hvez561XYcwDQYJKoZIhvcNAQELBQAw
gakxczBxBgNVBAsTam9wYy1kZXZpY2U6YjE6OWE6Yzg6MmM6ZDk6Nzc6YjI6ZTI6
Njk6NDc6OWY6Njg6OTE6ZDU6OGQ6MGQ6OGQ6ZGY6NmY6NWI6YmQ6ZjM6Mjc6NjA6
Yzg6OGU6MTM6YTk6OTQ6MjY6YTY6OWIxMjAwBgNVBAMTKVBLSVNWQyBJZGVudGl0
eSBJbnRlcm1lZGlhdGUgdXMtYXNoYnVybi0xMB4XDTIwMTExNjAwMjI1OFoXDTIw
MTExNjAyMjM1OFowggG8MVwwWgYDVQQDE1NvY2lkMS5pbnN0YW5jZS5vYzEuaWFk
LmFidXdjbGpycmtzeHFiN21teXA0aWdmdzdkbGs2MmZ0ZzNrcHk1YjZubmE1ZGdr
aXBpYXpmeGdjN2o3YTEeMBwGA1UECxMVb3BjLWNlcnR0eXBlOmluc3RhbmNlMWww
agYDVQQLE2NvcGMtY29tcGFydG1lbnQ6b2NpZDEuY29tcGFydG1lbnQub2MxLi5h
YWFhYWFhYXVtdG42NXljdWJmdTJsYjQybW82N3M3YmRjaTQ3ZDQ2aXJyNzIzZ3dy
ZDZ0NHV3YWppcXExaTBnBgNVBAsTYG9wYy1pbnN0YW5jZTpvY2lkMS5pbnN0YW5j
ZS5vYzEuaWFkLmFidXdjbGpycmtzeHFiN21teXA0aWdmdzdkbGs2MmZ0ZzNrcHk1
YjZubmE1ZGdraXBpYXpmeGdjN2o3YTFjMGEGA1UECxNab3BjLXRlbmFudDpvY2lk
MS50ZW5hbmN5Lm9jMS4uYWFhYWFhYWFoeTdyYzJwYTc0NnB4NHk2cHk2Z3B2dHB2
ZG1zcHd6Z2xkYWFpbW13Ym03NDJnazZzd3BhMIIBIjANBgkqhkiG9w0BAQEFAAOC
AQ8AMIIBCgKCAQEA0MIpsIyTWu6/4GbsN3hfqcbVoOmKwfXzqWMwITGDTAb8Sudy
BGD4JCKcB416+sREYw4s8gyJxvfb3kIiLnvI2CI3mPuywBguBkMyfAFApJk4a0zc
AlN2xds+u7/tyep6imMz6WZU+5NBiCqaVIr9k0rku7akiOw+YCacq0xt+4z+YM0S
5ra2R1AQnYG+j2bUfmOsnflHoLZx8TOiZKAXNjv1mg1AhxA+NIIuqT7/EvK/mEsV
Z78jneTP1K+hz1hK+AX1F4X7uhGP5VEbiD+ZNs+bOTupDBnQFiTYrKQM3b2qpNBv
cRPYQK8z+CxCJBPei17L8iqfYtj/9Ab+5p+yGwIDAQABo4ICDTCCAgkwEwYDVR0l
BAwwCgYIKwYBBQUHAwIwHwYDVR0jBBgwFoAUslrNVlexQg5CXpVM7MQRgrxQojkw
ggHPBgkrBgEEAW9iCgEEggHAMIIBvIEIaW5zdGFuY2WCU29jaWQxLmluc3RhbmNl
Lm9jMS5pYWQuYWJ1d2NsanJya3N4cWI3bW15cDRpZ2Z3N2RsazYyZnRnM2tweTVi
Nm5uYTVkZ2tpcGlhemZ4Z2M3ajdhg1NvY2lkMS5jb21wYXJ0bWVudC5vYzEuLmFh
YWFhYWFhdW10bjY1eWN1YmZ1MmxiNDJtbzY3czdiZGNpNDdkNDZpcnI3MjNnd3Jk
NnQ0dXdhamlxcYRPb2NpZDEudGVuYW5jeS5vYzEuLmFhYWFhYWFhaHk3cmMycGE3
NDZweDR5NnB5NmdwdnRwdmRtc3B3emdsZGFhaW1td2JtNzQyZ2s2c3dwYYWBtEFR
RUNBUitMQ0FBQUFBQUFBQUJqcWxZS1N5MHF6c3pQVTdJeTFGSHlMUFlyemNseEsw
cE5UY3N2eWcxSlRDOVdzaW9wS2syRnliaWtwbVhtcGFaQUpOSVNjNHFCTWluSVl0
VzFPa3JCT2FYcEFVWDVCYWxGSlptcElERWxrSnhYY1g1ZWNHWlZxbWVlVTJVSlNG
akpTTWRJcVJabWNuQmxjVWtxa28yMUFOYk5ONUNhQUFBQTANBgkqhkiG9w0BAQsF
AAOCAgEABy3PleJQWzPc8f8YtHgoc/Dlt6ND06tNZsigQJzI/pNcbn6BoIr1DFsN
phr6931xsExczX/2PALAyYZj9fJeTqvrmyvVrIIxt3F8hrvw3t5evS7zIQVCSXXy
8upsypTfIc9okQfv3v3yYJ1MfNtxI+bNYB1B/FaJIN+m9NfU8K2efDqtdqx4McAM
oe58D6XjxkR88XPjPhGp4j/UvDWL/UPkb8b82/+udwhM1h11VTicRV+L+oKbWTU3
IuOdFE3xfM5il4nXYNm6OcV9suL/m8VsXTEeuXf4Bl16DbcVJgTYDKX3NG9OJGr8
BAm9Q9bg42eqcJpIcXcA5N2nuYpDRXnmGqbwtb1QVYw4fAaOzydPG6gCly0ulQ1N
hqGaCaLbLIyjR1zmi5dSmU41v5A9cjplTazV9P6ipeiarkJ++s+92p2rYtz30VaR
9aRqEDEYJ+DKJa3A4B62tviu7kqJcdV1CMTZd/XUbbVHOYSUgjBuQE5XKIggJsi9
J7Aq2HX0jdquL2kCYt3vGBTiT83vhWP8RFw7qNQjyxsU9PDr60JTGR6CudTA6HYG
tnJYpvF7nMM/Kf+wnqcajEg1kLHzjdENxhfThvOH96ZwDVQBMTirzjeL0yEWOrlA
P4xtSxIkk0+SssUrMTUXv7FAKzRRNtbqArVIMKmTqihFEjek/gk=
-----END CERTIFICATE-----`
)

const (
	iaasInfoResponseFilePath                                       = "testdata/iaasInfoResponse.json"
	instanceResponseFilePath                                       = "testdata/instanceResponse.json"
	instanceResponseWithoutSubclassFilePath                        = "testdata/instanceResponseWithoutHostsubclass.json"
	instanceResponseWithInvalidHostSubclassFilePath                = "testdata/instanceResponseWithInvalidHostSubclass.json"
	instanceResponseWithHostSubclassNameWithCapitalLettersFilePath = "testdata/instanceResponseWithHostSubclassNameInCaps.json"
)

type fakeHttpDoer struct {
	instanceResponse         *http.Response
	certResponse             *http.Response
	intermediateCertResponse *http.Response
	iaasInfoResponse         *http.Response
	keyResponse              *http.Response
	err                      error
}

func (f *fakeHttpDoer) Do(req *http.Request) (*http.Response, error) {
	if f.err != nil {
		return nil, f.err
	}

	if strings.Contains(req.URL.RequestURI(), "identity/cert") {
		if strings.Contains(req.URL.RequestURI(), "v1") {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(sampleInstanceCert))}, nil
		}
		return f.certResponse, nil
	}

	if strings.Contains(req.URL.RequestURI(), "identity/key") {
		return f.keyResponse, nil
	}

	if strings.Contains(req.URL.RequestURI(), "identity/intermediate") {
		return f.intermediateCertResponse, nil
	}

	if strings.Contains(req.URL.RequestURI(), "instance") {
		if strings.Contains(req.URL.RequestURI(), "v1") {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBuffer(getFileBytes(instanceResponseFilePath)))}, nil
		}
		return f.instanceResponse, nil
	}

	if strings.Contains(req.URL.RequestURI(), "iaasInfo") {
		if strings.Contains(req.URL.RequestURI(), "v1") {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBuffer(getFileBytes(iaasInfoResponseFilePath)))}, nil
		}
		return f.iaasInfoResponse, nil
	}

	return nil, nil
}

func TestConfig_Validate(t *testing.T) {
	c := DefaultConfig()

	t.Run("happy-case", func(t *testing.T) {
		assert.NoError(t, c.Validate())
	})

	t.Run("nil", func(t *testing.T) {
		var c *Config
		assert.Error(t, c.Validate(), "should fail validation if nil")
	})

	t.Run("base-endpoint", func(t *testing.T) {
		c := c // shallow copy
		c.BaseEndpoint = ""
		assert.Error(t, c.Validate(), "should fail validation")
	})

	t.Run("timeout-after", func(t *testing.T) {
		c := c // shallow copy
		c.TimeoutAfter = -1
		assert.Error(t, c.Validate(), "should fail validation")
	})

	t.Run("auth-header-key", func(t *testing.T) {
		c := c // shallow copy
		c.AuthHeaderKey = ""
		assert.Error(t, c.Validate(), "should fail validation")
	})

	t.Run("auth-header-value", func(t *testing.T) {
		c := c // shallow copy
		c.AuthHeaderValue = ""
		assert.Error(t, c.Validate(), "should fail validation")
	})

	t.Run("instance-endpoint-suffix", func(t *testing.T) {
		c := c // shallow copy
		c.InstanceEndpointSuffix = ""
		assert.Error(t, c.Validate(), "should fail validation")
	})

	t.Run("iaas_info_endpoint_suffix", func(t *testing.T) {
		c := c // shallow copy
		c.IaasInfoEndpointSuffix = ""
		assert.Error(t, c.Validate(), "should fail validation")
	})
}

func TestImdsProvider(t *testing.T) {
	t.Run("happy case", func(t *testing.T) {
		imdsConfig := DefaultConfig()
		imdsConfig.TimeoutAfter = 10 * time.Second

		client := &fakeHttpDoer{
			certResponse: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(sampleInstanceCert)),
			},
			instanceResponse: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBuffer(getFileBytes(instanceResponseFilePath))),
			},
			iaasInfoResponse: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBuffer(getFileBytes(iaasInfoResponseFilePath))),
			},

			err: nil,
		}

		imdsClient := &Client{
			config:     imdsConfig,
			httpClient: client,
			logger:     logging.NewTestLogger(),
		}

		t.Run("get region from v1", func(t *testing.T) {
			region, err := imdsClient.GetRegion()
			assert.NoError(t, err)
			assert.Equal(t, "us-ashburn-1", region)
		})

		t.Run("get realm from v1", func(t *testing.T) {
			realm, err := imdsClient.GetRealm()
			assert.NoError(t, err)
			assert.Equal(t, "oc1", realm)
		})

		t.Run("get compartment from v2", func(t *testing.T) {
			compartmentID, err := imdsClient.GetCompartmentID()
			assert.NoError(t, err)
			assert.Equal(t, "ocid1.compartment.oc1..aaaaaaaanpl7hs5aijorrea4jn72oe47gfoe4tauerrqpxamk7cn7mde3req", compartmentID)
		})

		t.Run("get tenancy from v2", func(t *testing.T) {
			tenancyID, err := imdsClient.GetTenancyID()
			assert.NoError(t, err)
			assert.Equal(t, "ocid1.tenancy.oc1..aaaaaaaahy7rc2pa746px4y6py6gpvtpvdmspwzgldaaimmwbm742gk6swpa", tenancyID)
		})

		t.Run("get internal realm from v2", func(t *testing.T) {
			internalRealm, err := imdsClient.GetInternalRealmTLD()
			assert.NoError(t, err)
			assert.Equal(t, "oracleiaas.com", internalRealm)
		})

		t.Run("get hostclass from v2", func(t *testing.T) {
			hostclass, err := imdsClient.GetHostclass()
			assert.NoError(t, err)
			assert.Equal(t, "PERMISSIONS-SERVICE-UNSTABLE-OCICORP", hostclass)
		})

		t.Run("get host subclass from v2", func(t *testing.T) {
			subclass, err := imdsClient.GetHostSubclass()
			assert.NoError(t, err)
			assert.Equal(t, "customer42", subclass)
		})

	})

	t.Run("case for cert and key", func(t *testing.T) {
		imdsConfig := DefaultConfig()
		imdsConfig.TimeoutAfter = 10 * time.Second

		client := &fakeHttpDoer{
			certResponse: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(sampleInstanceCert)),
			},
			err: nil,
		}

		imdsClient := &Client{
			config:     imdsConfig,
			httpClient: client,
			logger:     logging.NewTestLogger(),
		}

		cert, err := imdsClient.GetCertificate()
		assert.NoError(t, err)
		actualCert := parseX509Cert(t)
		assert.True(t, cert.Equal(actualCert))

	})

	t.Run("case for key", func(t *testing.T) {
		imdsConfig := DefaultConfig()
		imdsConfig.TimeoutAfter = 10 * time.Second
		privKeyString := generateFakePrivateKey(t)
		client := &fakeHttpDoer{
			certResponse: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(sampleInstanceCert)),
			},
			keyResponse: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(privKeyString)),
			},

			err: nil,
		}

		imdsClient := &Client{
			config:     imdsConfig,
			httpClient: client,
			logger:     logging.NewTestLogger(),
		}

		// not checking the cert here since mocked response does not have that
		key, cert, err := imdsClient.GetX509KeyPair()
		assert.NoError(t, err)
		certBytes := []byte(sampleInstanceCert)
		keyBytes := []byte(privKeyString)
		assert.Equal(t, cert, certBytes)
		assert.Equal(t, key, keyBytes)

	})

	t.Run("case for missing subclass", func(t *testing.T) {
		imdsConfig := DefaultConfig()
		imdsConfig.TimeoutAfter = 10 * time.Second

		client := &fakeHttpDoer{
			instanceResponse: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBuffer(getFileBytes(instanceResponseWithoutSubclassFilePath))),
			},

			err: nil,
		}

		imdsClient := &Client{
			config:     imdsConfig,
			httpClient: client,
			logger:     logging.NewTestLogger(),
		}

		t.Run("no host subclass in response", func(t *testing.T) {
			subclass, err := imdsClient.GetHostSubclass()
			assert.Error(t, err)
			assert.Equal(t, "", subclass)
		})
	})

	t.Run("invalid host subclass name", func(t *testing.T) {
		imdsConfig := DefaultConfig()
		imdsConfig.TimeoutAfter = 10 * time.Second

		client := &fakeHttpDoer{
			instanceResponse: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBuffer(getFileBytes(instanceResponseWithInvalidHostSubclassFilePath))),
			},

			err: nil,
		}

		imdsClient := &Client{
			config:     imdsConfig,
			httpClient: client,
			logger:     logging.NewTestLogger(),
		}

		subclass, err := imdsClient.GetHostSubclass()
		assert.Error(t, err)
		assert.Equal(t, subclass, "")
	})

	t.Run("host subclass name has capital letters -- return host subclass in lower", func(t *testing.T) {
		imdsConfig := DefaultConfig()
		imdsConfig.TimeoutAfter = 10 * time.Second

		client := &fakeHttpDoer{
			instanceResponse: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBuffer(getFileBytes(instanceResponseWithHostSubclassNameWithCapitalLettersFilePath))),
			},

			err: nil,
		}

		imdsClient := &Client{
			config:     imdsConfig,
			httpClient: client,
			logger:     logging.NewTestLogger(),
		}

		subclass, err := imdsClient.GetHostSubclass()
		assert.NoError(t, err)
		assert.Equal(t, subclass, "test-subclass")
	})

	t.Run("case for intermediate cert", func(t *testing.T) {
		imdsConfig := DefaultConfig()
		imdsConfig.TimeoutAfter = 10 * time.Second
		client := &fakeHttpDoer{
			intermediateCertResponse: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(sampleInstanceCert)),
			},
			err: nil,
		}

		imdsClient := &Client{
			config:     imdsConfig,
			httpClient: client,
			logger:     logging.NewTestLogger(),
		}

		cert, err := imdsClient.GetIntermediateCertificate()
		assert.NoError(t, err)
		certBytes := []byte(sampleInstanceCert)
		assert.Equal(t, cert, certBytes)
	})

	t.Run("error case", func(t *testing.T) {
		imdsConfig := DefaultConfig()
		imdsConfig.TimeoutAfter = 10 * time.Second

		client := &fakeHttpDoer{
			err: errors.New("error from IMDS client"),
		}

		imdsClient := &Client{
			config:     imdsConfig,
			httpClient: client,
			logger:     logging.NewTestLogger(),
		}

		_, err := imdsClient.GetRegion()
		assert.Error(t, err)

		_, err = imdsClient.GetRealm()
		assert.Error(t, err)

		_, err = imdsClient.GetCompartmentID()
		assert.Error(t, err)

		_, err = imdsClient.GetInternalRealmTLD()
		assert.Error(t, err)

		_, err = imdsClient.GetHostclass()
		assert.Error(t, err)

		_, err = imdsClient.GetHostSubclass()
		assert.Error(t, err)

		_, err = imdsClient.GetIntermediateCertificate()
		assert.Error(t, err)
	})
}

func getFileBytes(fileName string) []byte {
	// Open the file
	file, _ := os.Open(fileName)

	// defer the closing of our jsonFile so that we can parse it later on
	defer func() {
		_ = file.Close()
	}()

	// read file as a byte array.
	byteValue, _ := io.ReadAll(file)

	return byteValue
}

func generateFakePrivateKey(t *testing.T) string {
	t.Helper()
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	require.NoError(t, privKey.Validate())

	privDer := x509.MarshalPKCS1PrivateKey(privKey)
	privBlock := &pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDer,
	}
	var privateKey bytes.Buffer
	require.NoError(t, pem.Encode(&privateKey, privBlock))
	return privateKey.String()
}

func parseX509Cert(t *testing.T) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode([]byte(sampleInstanceCert))
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	return cert
}
