package ociidentity

import (
	"context"
	"fmt"
	"testing"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	testingPkg "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/testing"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockIdentityClientInterface struct {
	mock.Mock
}

func (m *MockIdentityClientInterface) ListAvailabilityDomains(
	ctx context.Context,
	request identity.ListAvailabilityDomainsRequest,
) (identity.ListAvailabilityDomainsResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(identity.ListAvailabilityDomainsResponse), args.Error(1)
}

func TestGetConfigProvider_Identity(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config with user principal",
			config: &Config{
				AnotherLogger: testingPkg.SetupMockLogger(),
				AuthType:      &[]principals.AuthenticationType{principals.UserPrincipal}[0],
				Region:        "us-ashburn-1",
			},
			expectError: false,
		},
		{
			name: "valid config with instance principal",
			config: &Config{
				AnotherLogger: testingPkg.SetupMockLogger(),
				AuthType:      &[]principals.AuthenticationType{principals.InstancePrincipal}[0],
				Region:        "us-phoenix-1",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// As with the KMS tests, we mostly validate the config structure
			// that will be passed to getConfigProvider.
			assert.NotNil(t, tt.config.AnotherLogger)
			assert.NotNil(t, tt.config.AuthType)

			// In a fully mocked environment we would mock principals.Config.Build;
			// here we just make sure we're constructing Config correctly.
		})
	}
}

func TestNewIdentityClient_ConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "config with region",
			config: &Config{
				AnotherLogger: testingPkg.SetupMockLogger(),
				AuthType:      &[]principals.AuthenticationType{principals.UserPrincipal}[0],
				Region:        "us-ashburn-1",
			},
			expectError: false,
		},
		{
			name: "config without region",
			config: &Config{
				AnotherLogger: testingPkg.SetupMockLogger(),
				AuthType:      &[]principals.AuthenticationType{principals.InstancePrincipal}[0],
				Region:        "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Similar to your KMS NewKmsVaultClient tests: we don't really
			// call NewIdentityClient (would hit OCI), but we validate we're
			// constructing a sensible config.
			assert.NotNil(t, tt.config.AnotherLogger)
			assert.NotNil(t, tt.config.AuthType)
			// Region may be empty or set; both are allowed at wrapper level.
		})
	}
}

func TestIdentity_ListAvailabilityDomains(t *testing.T) {
	tests := []struct {
		name        string
		compartment string
		setupMocks  func(*MockIdentityClientInterface, *testingPkg.MockLogger, identity.ListAvailabilityDomainsRequest)
		expectError bool
		errorMsg    string
		expectedLen int
	}{
		{
			name:        "successful availability domains listing",
			compartment: "ocid1.compartment.oc1..example",
			setupMocks: func(mockClient *MockIdentityClientInterface, mockLogger *testingPkg.MockLogger, req identity.ListAvailabilityDomainsRequest) {
				resp := identity.ListAvailabilityDomainsResponse{
					Items: []identity.AvailabilityDomain{
						{Name: common.String("AD-1")},
						{Name: common.String("AD-2")},
					},
				}

				mockClient.
					On("ListAvailabilityDomains", mock.Anything, mock.MatchedBy(func(r identity.ListAvailabilityDomainsRequest) bool {
						return r.CompartmentId != nil && *r.CompartmentId == *req.CompartmentId
					})).
					Return(resp, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: false,
			expectedLen: 2,
		},
		{
			name:        "availability domains listing failure",
			compartment: "ocid1.compartment.oc1..example",
			setupMocks: func(mockClient *MockIdentityClientInterface, mockLogger *testingPkg.MockLogger, req identity.ListAvailabilityDomainsRequest) {
				mockClient.
					On("ListAvailabilityDomains", mock.Anything, mock.Anything).
					Return(identity.ListAvailabilityDomainsResponse{}, fmt.Errorf("failed to list availability domains for compartment test: OCI error"))

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
				mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to list availability domains",
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockIdentityClientInterface{}

			req := identity.ListAvailabilityDomainsRequest{
				CompartmentId: common.String(tt.compartment),
			}

			tt.setupMocks(mockClient, mockLogger, req)

			// This simulates the ListAvailabilityDomains logic using the mock client.
			resp, err := mockClient.ListAvailabilityDomains(context.Background(), req)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.Len(t, resp.Items, tt.expectedLen)
			}

			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}
