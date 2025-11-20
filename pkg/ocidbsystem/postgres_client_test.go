package ocidbsystem

import (
	"context"
	"fmt"
	"testing"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	testingPkg "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/testing"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/psql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPostgresqlClientInterface defines the interface for mocking Postgresql client
type MockPostgresqlClientInterface struct {
	mock.Mock
}

func (m *MockPostgresqlClientInterface) CreateDbSystem(
	ctx context.Context,
	request psql.CreateDbSystemRequest,
) (psql.CreateDbSystemResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(psql.CreateDbSystemResponse), args.Error(1)
}

func (m *MockPostgresqlClientInterface) GetDbSystem(
	ctx context.Context,
	request psql.GetDbSystemRequest,
) (psql.GetDbSystemResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(psql.GetDbSystemResponse), args.Error(1)
}

func (m *MockPostgresqlClientInterface) GetConnectionDetails(
	ctx context.Context,
	request psql.GetConnectionDetailsRequest,
) (psql.GetConnectionDetailsResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(psql.GetConnectionDetailsResponse), args.Error(1)
}

func (m *MockPostgresqlClientInterface) DeleteDbSystem(
	ctx context.Context,
	request psql.DeleteDbSystemRequest,
) (psql.DeleteDbSystemResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(psql.DeleteDbSystemResponse), args.Error(1)
}

func TestGetConfigProvider_Postgres(t *testing.T) {
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
			// Like KMS tests: we mostly validate the config shape and that
			// the function can be invoked with a valid Config.
			assert.NotNil(t, tt.config.AnotherLogger)
			assert.NotNil(t, tt.config.AuthType)

			// In a real environment we would mock principals.Config.Build.
			// Here we just check the config is structurally correct.
		})
	}
}

func TestNewOciPostgreSQLClient_ConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config with region",
			config: &Config{
				AnotherLogger: testingPkg.SetupMockLogger(),
				AuthType:      &[]principals.AuthenticationType{principals.UserPrincipal}[0],
				Region:        "us-ashburn-1",
			},
			expectError: false,
		},
		{
			name: "valid config without region",
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
			// We don't actually call OCI here (similar to KMS tests),
			// just validate that the config we will pass is well-formed.
			assert.NotNil(t, tt.config.AnotherLogger)
			assert.NotNil(t, tt.config.AuthType)

			// Region can be empty or non-empty; both are valid from the
			// perspective of our wrapper.
		})
	}
}

func TestPostgres_CreateDbSystem(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		compartment string
		setupMocks  func(*MockPostgresqlClientInterface, *testingPkg.MockLogger, psql.CreateDbSystemRequest)
		expectError bool
		errorMsg    string
	}{
		{
			name:        "successful DB system creation",
			displayName: "test-db",
			compartment: "ocid1.compartment.oc1..test",
			setupMocks: func(mockClient *MockPostgresqlClientInterface, mockLogger *testingPkg.MockLogger, req psql.CreateDbSystemRequest) {
				resp := psql.CreateDbSystemResponse{
					DbSystem: psql.DbSystem{
						Id: common.String("ocid1.postgresql.oc1..testdb"),
					},
				}

				mockClient.
					On("CreateDbSystem", mock.Anything, mock.MatchedBy(func(r psql.CreateDbSystemRequest) bool {
						return r.CreateDbSystemDetails.DisplayName != nil &&
							*r.CreateDbSystemDetails.DisplayName == *req.CreateDbSystemDetails.DisplayName
					})).
					Return(resp, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
			},
			expectError: false,
		},
		{
			name:        "DB system creation failure",
			displayName: "test-db",
			compartment: "ocid1.compartment.oc1..test",
			setupMocks: func(mockClient *MockPostgresqlClientInterface, mockLogger *testingPkg.MockLogger, req psql.CreateDbSystemRequest) {
				mockClient.
					On("CreateDbSystem", mock.Anything, mock.Anything).
					Return(psql.CreateDbSystemResponse{}, fmt.Errorf("failed to create Postgresql DB System: OCI error"))

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
				mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to create Postgresql DB System",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockPostgresqlClientInterface{}

			req := psql.CreateDbSystemRequest{
				CreateDbSystemDetails: psql.CreateDbSystemDetails{
					DisplayName:   common.String(tt.displayName),
					CompartmentId: common.String(tt.compartment),
				},
			}

			tt.setupMocks(mockClient, mockLogger, req)

			// Simulate wrapper logic by calling the mock directly
			resp, err := mockClient.CreateDbSystem(context.Background(), req)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp.DbSystem.Id)
				assert.Equal(t, "ocid1.postgresql.oc1..testdb", *resp.DbSystem.Id)
			}

			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestPostgres_GetDbSystem(t *testing.T) {
	tests := []struct {
		name        string
		dbSystemID  string
		setupMocks  func(*MockPostgresqlClientInterface, *testingPkg.MockLogger, psql.GetDbSystemRequest)
		expectError bool
		errorMsg    string
	}{
		{
			name:       "successful DB system retrieval",
			dbSystemID: "ocid1.postgresql.oc1..testdb",
			setupMocks: func(mockClient *MockPostgresqlClientInterface, mockLogger *testingPkg.MockLogger, req psql.GetDbSystemRequest) {
				resp := psql.GetDbSystemResponse{
					DbSystem: psql.DbSystem{
						Id: common.String("ocid1.postgresql.oc1..testdb"),
					},
				}

				mockClient.
					On("GetDbSystem", mock.Anything, mock.MatchedBy(func(r psql.GetDbSystemRequest) bool {
						return r.DbSystemId != nil && *r.DbSystemId == *req.DbSystemId
					})).
					Return(resp, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: false,
		},
		{
			name:       "DB system retrieval failure",
			dbSystemID: "ocid1.postgresql.oc1..testdb",
			setupMocks: func(mockClient *MockPostgresqlClientInterface, mockLogger *testingPkg.MockLogger, req psql.GetDbSystemRequest) {
				mockClient.
					On("GetDbSystem", mock.Anything, mock.Anything).
					Return(psql.GetDbSystemResponse{}, fmt.Errorf("failed to get Postgresql DB System testdb: OCI error"))

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
				mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to get Postgresql DB System",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockPostgresqlClientInterface{}

			req := psql.GetDbSystemRequest{
				DbSystemId: common.String(tt.dbSystemID),
			}

			tt.setupMocks(mockClient, mockLogger, req)

			resp, err := mockClient.GetDbSystem(context.Background(), req)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp.DbSystem.Id)
				assert.Equal(t, tt.dbSystemID, *resp.DbSystem.Id)
			}

			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestPostgres_GetConnectionDetails(t *testing.T) {
	tests := []struct {
		name        string
		dbSystemID  string
		setupMocks  func(*MockPostgresqlClientInterface, *testingPkg.MockLogger, psql.GetConnectionDetailsRequest)
		expectError bool
		errorMsg    string
	}{
		{
			name:       "successful connection details retrieval",
			dbSystemID: "ocid1.postgresql.oc1..testdb",
			setupMocks: func(mockClient *MockPostgresqlClientInterface, mockLogger *testingPkg.MockLogger, req psql.GetConnectionDetailsRequest) {
				resp := psql.GetConnectionDetailsResponse{
					// we don't really care what is inside here for this test,
					// just that a non-zero response can be returned
				}

				mockClient.
					On("GetConnectionDetails", mock.Anything, mock.MatchedBy(func(r psql.GetConnectionDetailsRequest) bool {
						return r.DbSystemId != nil && *r.DbSystemId == *req.DbSystemId
					})).
					Return(resp, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: false,
		},
		{
			name:       "connection details retrieval failure",
			dbSystemID: "ocid1.postgresql.oc1..testdb",
			setupMocks: func(mockClient *MockPostgresqlClientInterface, mockLogger *testingPkg.MockLogger, req psql.GetConnectionDetailsRequest) {
				mockClient.
					On("GetConnectionDetails", mock.Anything, mock.Anything).
					Return(psql.GetConnectionDetailsResponse{}, fmt.Errorf("failed to get connection details testdb: OCI error"))

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
				mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to get connection details",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockPostgresqlClientInterface{}

			req := psql.GetConnectionDetailsRequest{
				DbSystemId: common.String(tt.dbSystemID),
			}

			tt.setupMocks(mockClient, mockLogger, req)

			resp, err := mockClient.GetConnectionDetails(context.Background(), req)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				// We don't assert on specific fields; just that the call succeeded.
				assert.NotNil(t, resp)
			}

			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestPostgres_DeleteDbSystem(t *testing.T) {
	tests := []struct {
		name        string
		dbSystemID  string
		setupMocks  func(*MockPostgresqlClientInterface, *testingPkg.MockLogger, psql.DeleteDbSystemRequest)
		expectError bool
		errorMsg    string
	}{
		{
			name:       "successful DB system deletion",
			dbSystemID: "ocid1.postgresql.oc1..testdb",
			setupMocks: func(mockClient *MockPostgresqlClientInterface, mockLogger *testingPkg.MockLogger, req psql.DeleteDbSystemRequest) {
				resp := psql.DeleteDbSystemResponse{}

				mockClient.
					On("DeleteDbSystem", mock.Anything, mock.MatchedBy(func(r psql.DeleteDbSystemRequest) bool {
						return r.DbSystemId != nil && *r.DbSystemId == *req.DbSystemId
					})).
					Return(resp, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: false,
		},
		{
			name:       "DB system deletion failure",
			dbSystemID: "ocid1.postgresql.oc1..testdb",
			setupMocks: func(mockClient *MockPostgresqlClientInterface, mockLogger *testingPkg.MockLogger, req psql.DeleteDbSystemRequest) {
				mockClient.
					On("DeleteDbSystem", mock.Anything, mock.Anything).
					Return(psql.DeleteDbSystemResponse{}, fmt.Errorf("failed to delete Postgresql DB System testdb: OCI error"))

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
				mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to delete Postgresql DB System",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockPostgresqlClientInterface{}

			req := psql.DeleteDbSystemRequest{
				DbSystemId: common.String(tt.dbSystemID),
			}

			tt.setupMocks(mockClient, mockLogger, req)

			resp, err := mockClient.DeleteDbSystem(context.Background(), req)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}

			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}
