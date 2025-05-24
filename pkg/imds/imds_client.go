package imds

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/sgl-project/sgl-ome/pkg/logging"
)

const (
	opcTenantPrefix = "opc-tenant:"
)

var validHostSubclassRegex = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9-]{2,}$")

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// instanceResult stores the information that we fetched
// from baseUrl+InstanceEndpointSuffix.
type instanceResult struct {
	Realm         string
	Region        string
	CompartmentId string
	Hostclass     string
	HostSubclass  string
	Shape         string
}

// instanceResult stores the information that we fetched
// from baseUrl+IaasInfoEndpointSuffix.
type iaasInfoResult struct {
	IaasDomainName   string
	PublicDomainName string
}

type Client struct {
	config     Config
	httpClient httpDoer
	logger     logging.Interface

	// Instead of doing a request to get each piece of data,
	// it's better to pull all 3 pieces at once (region/realm/compartmentId)
	// and save to instanceResult.
	// NOTE: GetTenancyId still requires pulling & parsing the instance certificate,
	// but nothing we can do here. Nobody really uses this variable anyway.
	instanceResultOnce sync.Once
	instanceResult     instanceResult
	iaasInfoResult     iaasInfoResult
	instanceResultErr  error
	iassInfoResultErr  error
}

// NewClient creates a new imds client.
func NewClient(config Config, logger logging.Interface) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validating imds config: %w", err)
	}

	return &Client{
		config: config,
		logger: logger,
		httpClient: &http.Client{
			Timeout: config.TimeoutAfter,
		},
	}, nil
}

// GetRealm fetches realm from the instance endpoint response.
func (ip *Client) GetRealm() (string, error) {
	ir, err := ip.getInstanceResult()
	if err != nil {
		return "", err
	}

	return ir.Realm, nil
}

// GetRegion fetches region from the instance endpoint response.
func (ip *Client) GetRegion() (string, error) {
	ir, err := ip.getInstanceResult()
	if err != nil {
		return "", err
	}

	return ir.Region, nil
}

// GetTenancyID fetches tenancy ID from the identity certificate endpoint.
func (ip *Client) GetTenancyID() (string, error) {
	cert, err := ip.GetCertificate()
	if err != nil {
		return "", err
	}

	return extractOU(cert, opcTenantPrefix)
}

// GetCompartmentID fetches instance compartment from the instance endpoint
// response.
func (ip *Client) GetCompartmentID() (string, error) {
	ir, err := ip.getInstanceResult()
	if err != nil {
		return "", err
	}

	return ir.CompartmentId, nil
}

// GetRealmTLD fetches Realm's Top-level Domain from the iaas info endpoint
// response.
func (ip *Client) GetRealmTLD() (string, error) {
	infoResult, err := ip.getIaasInfoResultJson()
	if err != nil {
		return "", err
	}

	return infoResult.PublicDomainName, nil
}

// GetInternalRealmTLD fetches Realm's internal Top-level Domain from the iaas
// info endpoint response.
func (ip *Client) GetInternalRealmTLD() (string, error) {
	infoResult, err := ip.getIaasInfoResultJson()
	if err != nil {
		return "", err
	}

	return infoResult.IaasDomainName, nil
}

// GetHostSubclass fetches Host subclass from the instance endpoint response.
func (ip *Client) GetHostSubclass() (string, error) {
	ir, err := ip.getInstanceResult()
	if err != nil {
		return "", err
	}

	if !validHostSubclassRegex.MatchString(ir.HostSubclass) {
		return "", fmt.Errorf("invalid Host Subclass: %s", ir.HostSubclass)
	}
	return strings.ToLower(ir.HostSubclass), nil
}

// GetHostclass fetches hostclass from the instance endpoint response.
func (ip *Client) GetHostclass() (string, error) {
	ir, err := ip.getInstanceResult()
	if err != nil {
		return "", err
	}

	return ir.Hostclass, nil
}

// GetCertificate fetches the Identity leaf cert from IMDS.
func (ip *Client) GetCertificate() (*x509.Certificate, error) {
	pemCert, err := ip.getBytesWithFallback(ip.config.IdentityCertEndpointSuffix)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(pemCert)
	if block == nil {
		return nil, fmt.Errorf("no pem block found")
	}
	if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
		return nil, fmt.Errorf("invalid block type or block headers are not empty")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing certificate: %w", err)
	}

	return cert, nil
}

// GetX509KeyPair fetches the identity leaf certificate and its private key from IMDS.
func (ip *Client) GetX509KeyPair() (key []byte, leafCert []byte, err error) {
	leafCert, err = ip.getBytesWithFallback(ip.config.IdentityCertEndpointSuffix)
	if err != nil {
		return nil, nil, err
	}

	key, err = ip.getBytesWithFallback(ip.config.IdentityCertPrivateKeyEndpointSuffix)
	if err != nil {
		return nil, nil, err
	}

	return key, leafCert, nil
}

// GetIntermediateCertificate fetches the identity intermediate cert from IMDS.
func (ip *Client) GetIntermediateCertificate() ([]byte, error) {
	intermediateCert, err := ip.getBytesWithFallback(ip.config.IdentityIntermediateCertEndpointSuffix)
	if err != nil {
		return nil, err
	}

	return intermediateCert, nil
}

func (ip *Client) GetInstanceShape() (string, error) {
	ir, err := ip.getInstanceResult()
	if err != nil {
		return "", err
	}

	return ir.Shape, nil
}

func (ip *Client) getInstanceResult() (instanceResult, error) {
	ip.instanceResultOnce.Do(func() {
		jsonBytes, err := ip.getBytesWithFallback(ip.config.InstanceEndpointSuffix)
		if err != nil {
			ip.instanceResultErr = err
			return
		}

		ip.instanceResult, ip.instanceResultErr = parseInstanceResult(jsonBytes)
	})

	return ip.instanceResult, ip.instanceResultErr
}

// getIaasInfoResultJson would return the deserialized iaasInfoResult from IMDS.
func (ip *Client) getIaasInfoResultJson() (iaasInfoResult, error) {
	jsonBytes, err := ip.getBytesWithFallback(ip.config.IaasInfoEndpointSuffix)
	if err != nil {
		ip.iassInfoResultErr = err
		return iaasInfoResult{}, err
	}

	ip.iaasInfoResult, ip.iassInfoResultErr = parseIaasInfoResultJson(jsonBytes)

	return ip.iaasInfoResult, ip.iassInfoResultErr
}

func parseIaasInfoResultJson(jsonBytes []byte) (iaasInfoResult, error) {
	jsonValue := struct {
		Realm struct {
			IaasDomainName   string `json:"iaasDomainName"`
			PublicDomainName string `json:"publicDomainName"`
		} `json:"realm"`
	}{}

	if err := json.Unmarshal(jsonBytes, &jsonValue); err != nil {
		return iaasInfoResult{}, err
	}

	return iaasInfoResult{
		IaasDomainName:   jsonValue.Realm.IaasDomainName,
		PublicDomainName: jsonValue.Realm.PublicDomainName,
	}, nil
}

func extractOU(cert *x509.Certificate, prefix string) (string, error) {
	// Subject looks like:
	// /CN=ocid1.instance.oc1.iad.abuwcljrrksxqb7mmyp4igfw7dlk62ftg3kpy5b6nna5dgkipiazfxgc7j7a
	// /OU=opc-certtype:instance
	// /OU=opc-compartment:ocid1.compartment.oc1..aaaaaaaaumtn65ycubfu2lb42mo67s7bdci47d46irr723gwrd6t4uwajiqq
	// /OU=opc-instance:ocid1.instance.oc1.iad.abuwcljrrksxqb7mmyp4igfw7dlk62ftg3kpy5b6nna5dgkipiazfxgc7j7a
	// /OU=opc-tenant:ocid1.tenancy.oc1..aaaaaaaahy7rc2pa746px4y6py6gpvtpvdmspwzgldaaimmwbm742gk6swpa
	for _, unit := range cert.Subject.OrganizationalUnit {
		if strings.HasPrefix(unit, prefix) {
			return strings.TrimPrefix(unit, prefix), nil
		}
	}

	return "", fmt.Errorf("can't find OU with %s prefix", prefix)
}

// getBytes makes an HTTP GET request to the endpoint (base+suffix)
// If 404 is returned, then the fallback base endpoint is used.
func (ip *Client) getBytesWithFallback(path string) ([]byte, error) {
	statusCode, respBytes, err := ip.actuallyGetBytes(path, true /* v2 */)
	if err != nil || statusCode == 404 {
		ip.logger.
			WithField("fallback_endpoint", ip.config.FallbackBaseEndpoint).
			WithField("status_code", statusCode).
			WithError(err).
			Info("Falling back to v1 endpoint...")

		statusCode, respBytes, err = ip.actuallyGetBytes(path, false /* v2 */)
	}

	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode > 299 {
		return nil, fmt.Errorf("non-2xx error code (%d): %s", statusCode, string(respBytes))
	}

	return respBytes, nil
}

func (ip *Client) actuallyGetBytes(path string, v2 bool) (int, []byte, error) {
	req, err := ip.newGetRequest(context.Background(), path, v2)
	if err != nil {
		return 0, nil, err
	}

	ip.logger.
		WithField("request_method", req.Method).
		WithField("request_url", req.URL).
		Debug("Attempting to get ...")

	resp, err := ip.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get %q: %v", req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("failed to read response body: %v", err)
	}

	return resp.StatusCode, respBytes, nil
}

func (ip *Client) newGetRequest(ctx context.Context, path string, v2 bool) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ip.pathTo(path, v2), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	if v2 {
		if ip.config.AuthHeaderKey != "" && ip.config.AuthHeaderValue != "" {
			req.Header.Add(ip.config.AuthHeaderKey, ip.config.AuthHeaderValue)
		}
	}

	return req, err
}

func (ip *Client) pathTo(path string, v2 bool) string {
	endpoint := ip.config.BaseEndpoint
	if !v2 {
		endpoint = ip.config.FallbackBaseEndpoint
	}

	return endpoint + path
}
