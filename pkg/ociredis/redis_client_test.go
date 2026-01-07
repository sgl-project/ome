package ociredis

import (
	"context"
	"fmt"
	"testing"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/principals"
	testingPkg "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/testing"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRedisClusterClientInterface struct {
	mock.Mock
}

func (m *MockRedisClusterClientInterface) CreateRedisCluster(
	ctx context.Context,
	request redis.CreateRedisClusterRequest,
) (redis.CreateRedisClusterResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(redis.CreateRedisClusterResponse), args.Error(1)
}

func (m *MockRedisClusterClientInterface) GetRedisCluster(
	ctx context.Context,
	request redis.GetRedisClusterRequest,
) (redis.GetRedisClusterResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(redis.GetRedisClusterResponse), args.Error(1)
}

func (m *MockRedisClusterClientInterface) DeleteRedisCluster(
	ctx context.Context,
	request redis.DeleteRedisClusterRequest,
) (redis.DeleteRedisClusterResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(redis.DeleteRedisClusterResponse), args.Error(1)
}

func (m *MockRedisClusterClientInterface) AttachOciCacheUsers(
	ctx context.Context,
	request redis.AttachOciCacheUsersRequest,
) (redis.AttachOciCacheUsersResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(redis.AttachOciCacheUsersResponse), args.Error(1)
}

func (m *MockRedisClusterClientInterface) DetachOciCacheUsers(
	ctx context.Context,
	request redis.DetachOciCacheUsersRequest,
) (redis.DetachOciCacheUsersResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(redis.DetachOciCacheUsersResponse), args.Error(1)
}

type MockRedisUserClientInterface struct {
	mock.Mock
}

func (m *MockRedisUserClientInterface) CreateOciCacheUser(
	ctx context.Context,
	request redis.CreateOciCacheUserRequest,
) (redis.CreateOciCacheUserResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(redis.CreateOciCacheUserResponse), args.Error(1)
}

func (m *MockRedisUserClientInterface) GetOciCacheUser(
	ctx context.Context,
	request redis.GetOciCacheUserRequest,
) (redis.GetOciCacheUserResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(redis.GetOciCacheUserResponse), args.Error(1)
}

func (m *MockRedisUserClientInterface) DeleteOciCacheUser(
	ctx context.Context,
	request redis.DeleteOciCacheUserRequest,
) (redis.DeleteOciCacheUserResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(redis.DeleteOciCacheUserResponse), args.Error(1)
}

func TestGetConfigProvider_Redis(t *testing.T) {
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
			assert.NotNil(t, tt.config.AnotherLogger)
			assert.NotNil(t, tt.config.AuthType)
		})
	}
}

func TestNewOciRedisClient_ConfigValidation(t *testing.T) {
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
			assert.NotNil(t, tt.config.AnotherLogger)
			assert.NotNil(t, tt.config.AuthType)
			// Region can be empty or non-empty.
		})
	}
}

func TestRedis_CreateRedisCluster(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		compartment string
		setupMocks  func(*MockRedisClusterClientInterface, *testingPkg.MockLogger, redis.CreateRedisClusterRequest)
		expectError bool
		errorMsg    string
	}{
		{
			name:        "successful Redis cluster creation",
			displayName: "test-redis",
			compartment: "ocid1.compartment.oc1..test",
			setupMocks: func(mockClient *MockRedisClusterClientInterface, mockLogger *testingPkg.MockLogger, req redis.CreateRedisClusterRequest) {
				resp := redis.CreateRedisClusterResponse{
					RedisCluster: redis.RedisCluster{
						Id: common.String("ocid1.rediscluster.oc1..testredis"),
					},
				}

				mockClient.
					On("CreateRedisCluster", mock.Anything, mock.MatchedBy(func(r redis.CreateRedisClusterRequest) bool {
						return r.CreateRedisClusterDetails.DisplayName != nil &&
							*r.CreateRedisClusterDetails.DisplayName == *req.CreateRedisClusterDetails.DisplayName
					})).
					Return(resp, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
			},
			expectError: false,
		},
		{
			name:        "Redis cluster creation failure",
			displayName: "test-redis",
			compartment: "ocid1.compartment.oc1..test",
			setupMocks: func(mockClient *MockRedisClusterClientInterface, mockLogger *testingPkg.MockLogger, req redis.CreateRedisClusterRequest) {
				mockClient.
					On("CreateRedisCluster", mock.Anything, mock.Anything).
					Return(redis.CreateRedisClusterResponse{}, fmt.Errorf("failed to create Redis cluster test-redis: OCI error"))

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
				mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to create Redis cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockRedisClusterClientInterface{}

			req := redis.CreateRedisClusterRequest{
				CreateRedisClusterDetails: redis.CreateRedisClusterDetails{
					DisplayName:   common.String(tt.displayName),
					CompartmentId: common.String(tt.compartment),
				},
			}

			tt.setupMocks(mockClient, mockLogger, req)

			resp, err := mockClient.CreateRedisCluster(context.Background(), req)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp.RedisCluster.Id)
				assert.Equal(t, "ocid1.rediscluster.oc1..testredis", *resp.RedisCluster.Id)
			}

			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestRedis_GetRedisCluster(t *testing.T) {
	tests := []struct {
		name        string
		clusterID   string
		setupMocks  func(*MockRedisClusterClientInterface, *testingPkg.MockLogger, redis.GetRedisClusterRequest)
		expectError bool
		errorMsg    string
	}{
		{
			name:      "successful Redis cluster retrieval",
			clusterID: "ocid1.rediscluster.oc1..testredis",
			setupMocks: func(mockClient *MockRedisClusterClientInterface, mockLogger *testingPkg.MockLogger, req redis.GetRedisClusterRequest) {
				resp := redis.GetRedisClusterResponse{
					RedisCluster: redis.RedisCluster{
						Id: common.String("ocid1.rediscluster.oc1..testredis"),
					},
				}

				mockClient.
					On("GetRedisCluster", mock.Anything, mock.MatchedBy(func(r redis.GetRedisClusterRequest) bool {
						return r.RedisClusterId != nil && *r.RedisClusterId == *req.RedisClusterId
					})).
					Return(resp, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: false,
		},
		{
			name:      "Redis cluster retrieval failure",
			clusterID: "ocid1.rediscluster.oc1..testredis",
			setupMocks: func(mockClient *MockRedisClusterClientInterface, mockLogger *testingPkg.MockLogger, req redis.GetRedisClusterRequest) {
				mockClient.
					On("GetRedisCluster", mock.Anything, mock.Anything).
					Return(redis.GetRedisClusterResponse{}, fmt.Errorf("failed to get Redis cluster testredis: OCI error"))

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
				mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to get Redis cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockRedisClusterClientInterface{}

			req := redis.GetRedisClusterRequest{
				RedisClusterId: common.String(tt.clusterID),
			}

			tt.setupMocks(mockClient, mockLogger, req)

			resp, err := mockClient.GetRedisCluster(context.Background(), req)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp.RedisCluster.Id)
				assert.Equal(t, tt.clusterID, *resp.RedisCluster.Id)
			}

			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestRedis_DeleteRedisCluster(t *testing.T) {
	tests := []struct {
		name        string
		clusterID   string
		setupMocks  func(*MockRedisClusterClientInterface, *testingPkg.MockLogger, redis.DeleteRedisClusterRequest)
		expectError bool
		errorMsg    string
	}{
		{
			name:      "successful Redis cluster deletion",
			clusterID: "ocid1.rediscluster.oc1..testredis",
			setupMocks: func(mockClient *MockRedisClusterClientInterface, mockLogger *testingPkg.MockLogger, req redis.DeleteRedisClusterRequest) {
				resp := redis.DeleteRedisClusterResponse{}

				mockClient.
					On("DeleteRedisCluster", mock.Anything, mock.MatchedBy(func(r redis.DeleteRedisClusterRequest) bool {
						return r.RedisClusterId != nil && *r.RedisClusterId == *req.RedisClusterId
					})).
					Return(resp, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: false,
		},
		{
			name:      "Redis cluster deletion failure",
			clusterID: "ocid1.rediscluster.oc1..testredis",
			setupMocks: func(mockClient *MockRedisClusterClientInterface, mockLogger *testingPkg.MockLogger, req redis.DeleteRedisClusterRequest) {
				mockClient.
					On("DeleteRedisCluster", mock.Anything, mock.Anything).
					Return(redis.DeleteRedisClusterResponse{}, fmt.Errorf("failed to delete Redis cluster testredis: OCI error"))

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
				mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to delete Redis cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockRedisClusterClientInterface{}

			req := redis.DeleteRedisClusterRequest{
				RedisClusterId: common.String(tt.clusterID),
			}

			tt.setupMocks(mockClient, mockLogger, req)

			resp, err := mockClient.DeleteRedisCluster(context.Background(), req)
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

// ---- Redis user operations ----

func TestRedis_CreateUserWithPassword(t *testing.T) {
	tests := []struct {
		name            string
		compartmentOCID string
		userName        string
		description     string
		acl             string
		hashedPasswords []string
		setupMocks      func(*MockRedisUserClientInterface, *testingPkg.MockLogger, redis.CreateOciCacheUserRequest)
		expectError     bool
		errorMsg        string
	}{
		{
			name:            "successful user creation",
			compartmentOCID: "ocid1.compartment.oc1..test",
			userName:        "appA_user",
			description:     "test user",
			acl:             "~appA:* +@read +@write",
			hashedPasswords: []string{"hashed1"},
			setupMocks: func(mockClient *MockRedisUserClientInterface, mockLogger *testingPkg.MockLogger, req redis.CreateOciCacheUserRequest) {
				resp := redis.CreateOciCacheUserResponse{
					OciCacheUser: redis.OciCacheUser{
						Id:   common.String("ocid1.ocicacheuser.oc1..user1"),
						Name: common.String("appA_user"),
					},
				}

				mockClient.
					On("CreateOciCacheUser", mock.Anything, mock.MatchedBy(func(r redis.CreateOciCacheUserRequest) bool {
						d := r.CreateOciCacheUserDetails
						return d.CompartmentId != nil &&
							*d.CompartmentId == *req.CreateOciCacheUserDetails.CompartmentId &&
							d.Name != nil && *d.Name == *req.CreateOciCacheUserDetails.Name &&
							d.AclString != nil && *d.AclString == *req.CreateOciCacheUserDetails.AclString
					})).
					Return(resp, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Maybe()
			},
			expectError: false,
		},
		{
			name:            "user creation failure",
			compartmentOCID: "ocid1.compartment.oc1..test",
			userName:        "appA_user",
			description:     "test user",
			acl:             "~appA:* +@read +@write",
			hashedPasswords: []string{"hashed1"},
			setupMocks: func(mockClient *MockRedisUserClientInterface, mockLogger *testingPkg.MockLogger, req redis.CreateOciCacheUserRequest) {
				mockClient.
					On("CreateOciCacheUser", mock.Anything, mock.Anything).
					Return(redis.CreateOciCacheUserResponse{}, fmt.Errorf("failed to create cache user appA_user: OCI error"))

				mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to create cache user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockRedisUserClientInterface{}

			authMode := redis.PasswordAuthenticationMode{
				HashedPasswords: tt.hashedPasswords,
			}
			req := redis.CreateOciCacheUserRequest{
				CreateOciCacheUserDetails: redis.CreateOciCacheUserDetails{
					CompartmentId:      common.String(tt.compartmentOCID),
					Name:               common.String(tt.userName),
					Description:        common.String(tt.description),
					AclString:          common.String(tt.acl),
					AuthenticationMode: authMode,
					Status:             redis.OciCacheUserStatusOn,
				},
			}

			tt.setupMocks(mockClient, mockLogger, req)

			resp, err := mockClient.CreateOciCacheUser(context.Background(), req)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp.OciCacheUser.Id)
				assert.Equal(t, "appA_user", *resp.OciCacheUser.Name)
			}

			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestRedis_AttachUserToCluster(t *testing.T) {
	tests := []struct {
		name        string
		clusterOCID string
		userOCIDs   []string
		setupMocks  func(*MockRedisClusterClientInterface, *testingPkg.MockLogger, redis.AttachOciCacheUsersRequest)
		expectError bool
		errorMsg    string
	}{
		{
			name:        "successful attach users",
			clusterOCID: "ocid1.rediscluster.oc1..testredis",
			userOCIDs:   []string{"ocid1.ocicacheuser.oc1..user1", "ocid1.ocicacheuser.oc1..user2"},
			setupMocks: func(mockClient *MockRedisClusterClientInterface, mockLogger *testingPkg.MockLogger, req redis.AttachOciCacheUsersRequest) {
				resp := redis.AttachOciCacheUsersResponse{}

				mockClient.
					On("AttachOciCacheUsers", mock.Anything, mock.MatchedBy(func(r redis.AttachOciCacheUsersRequest) bool {
						return r.RedisClusterId != nil &&
							*r.RedisClusterId == *req.RedisClusterId &&
							len(r.AttachOciCacheUsersDetails.OciCacheUsers) == len(req.AttachOciCacheUsersDetails.OciCacheUsers)
					})).
					Return(resp, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: false,
		},
		{
			name:        "attach users failure",
			clusterOCID: "ocid1.rediscluster.oc1..testredis",
			userOCIDs:   []string{"ocid1.ocicacheuser.oc1..user1"},
			setupMocks: func(mockClient *MockRedisClusterClientInterface, mockLogger *testingPkg.MockLogger, req redis.AttachOciCacheUsersRequest) {
				mockClient.
					On("AttachOciCacheUsers", mock.Anything, mock.Anything).
					Return(redis.AttachOciCacheUsersResponse{}, fmt.Errorf("failed to attach users: OCI error"))

				mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to attach users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockRedisClusterClientInterface{}

			req := redis.AttachOciCacheUsersRequest{
				RedisClusterId: common.String(tt.clusterOCID),
				AttachOciCacheUsersDetails: redis.AttachOciCacheUsersDetails{
					OciCacheUsers: tt.userOCIDs,
				},
			}

			tt.setupMocks(mockClient, mockLogger, req)

			resp, err := mockClient.AttachOciCacheUsers(context.Background(), req)
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

func TestRedis_GetOciCacheUser(t *testing.T) {
	tests := []struct {
		name        string
		userOCID    string
		setupMocks  func(*MockRedisUserClientInterface, *testingPkg.MockLogger, redis.GetOciCacheUserRequest)
		expectError bool
		errorMsg    string
	}{
		{
			name:     "successful user get",
			userOCID: "ocid1.ocicacheuser.oc1..user1",
			setupMocks: func(mockClient *MockRedisUserClientInterface, mockLogger *testingPkg.MockLogger, req redis.GetOciCacheUserRequest) {
				resp := redis.GetOciCacheUserResponse{
					OciCacheUser: redis.OciCacheUser{
						Id:   common.String("ocid1.ocicacheuser.oc1..user1"),
						Name: common.String("appA_user"),
					},
				}

				mockClient.
					On("GetOciCacheUser", mock.Anything, mock.MatchedBy(func(r redis.GetOciCacheUserRequest) bool {
						return r.OciCacheUserId != nil && *r.OciCacheUserId == *req.OciCacheUserId
					})).
					Return(resp, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: false,
		},
		{
			name:     "user get failure",
			userOCID: "ocid1.ocicacheuser.oc1..user1",
			setupMocks: func(mockClient *MockRedisUserClientInterface, mockLogger *testingPkg.MockLogger, req redis.GetOciCacheUserRequest) {
				mockClient.
					On("GetOciCacheUser", mock.Anything, mock.Anything).
					Return(redis.GetOciCacheUserResponse{}, fmt.Errorf("failed to get cache user: OCI error"))

				mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to get cache user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockRedisUserClientInterface{}

			req := redis.GetOciCacheUserRequest{
				OciCacheUserId: common.String(tt.userOCID),
			}

			tt.setupMocks(mockClient, mockLogger, req)

			resp, err := mockClient.GetOciCacheUser(context.Background(), req)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp.OciCacheUser.Id)
				assert.Equal(t, tt.userOCID, *resp.OciCacheUser.Id)
			}

			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestRedis_DeleteOciCacheUser(t *testing.T) {
	tests := []struct {
		name        string
		userOCID    string
		setupMocks  func(*MockRedisUserClientInterface, *testingPkg.MockLogger, redis.DeleteOciCacheUserRequest)
		expectError bool
		errorMsg    string
	}{
		{
			name:     "successful user delete",
			userOCID: "ocid1.ocicacheuser.oc1..user1",
			setupMocks: func(mockClient *MockRedisUserClientInterface, mockLogger *testingPkg.MockLogger, req redis.DeleteOciCacheUserRequest) {
				resp := redis.DeleteOciCacheUserResponse{}

				mockClient.
					On("DeleteOciCacheUser", mock.Anything, mock.MatchedBy(func(r redis.DeleteOciCacheUserRequest) bool {
						return r.OciCacheUserId != nil && *r.OciCacheUserId == *req.OciCacheUserId
					})).
					Return(resp, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: false,
		},
		{
			name:     "user delete failure",
			userOCID: "ocid1.ocicacheuser.oc1..user1",
			setupMocks: func(mockClient *MockRedisUserClientInterface, mockLogger *testingPkg.MockLogger, req redis.DeleteOciCacheUserRequest) {
				mockClient.
					On("DeleteOciCacheUser", mock.Anything, mock.Anything).
					Return(redis.DeleteOciCacheUserResponse{}, fmt.Errorf("failed to delete cache user: OCI error"))

				mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to delete cache user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockRedisUserClientInterface{}

			req := redis.DeleteOciCacheUserRequest{
				OciCacheUserId: common.String(tt.userOCID),
			}

			tt.setupMocks(mockClient, mockLogger, req)

			resp, err := mockClient.DeleteOciCacheUser(context.Background(), req)
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

func TestRedis_DetachCacheUserFromCluster(t *testing.T) {
	tests := []struct {
		name        string
		userOCIDs   []string
		clusterID   string
		setupMocks  func(*MockRedisClusterClientInterface, *testingPkg.MockLogger, redis.DetachOciCacheUsersRequest)
		expectError bool
		errorMsg    string
	}{
		{
			name:      "successful detach users",
			userOCIDs: []string{"ocid1.ocicacheuser.oc1..user1", "ocid1.ocicacheuser.oc1..user2"},
			clusterID: "ocid1.rediscachecluster.oc1..cluster1",
			setupMocks: func(mockClient *MockRedisClusterClientInterface, mockLogger *testingPkg.MockLogger, req redis.DetachOciCacheUsersRequest) {
				resp := redis.DetachOciCacheUsersResponse{}

				mockClient.
					On("DetachOciCacheUsers", mock.Anything, mock.MatchedBy(func(r redis.DetachOciCacheUsersRequest) bool {
						if r.RedisClusterId == nil || *r.RedisClusterId != *req.RedisClusterId {
							return false
						}
						// details is a value type in SDK; compare the slice content
						return assert.ObjectsAreEqual(r.DetachOciCacheUsersDetails.OciCacheUsers, req.DetachOciCacheUsersDetails.OciCacheUsers)
					})).
					Return(resp, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: false,
		},
		{
			name:      "detach failure",
			userOCIDs: []string{"ocid1.ocicacheuser.oc1..user1"},
			clusterID: "ocid1.rediscachecluster.oc1..cluster1",
			setupMocks: func(mockClient *MockRedisClusterClientInterface, mockLogger *testingPkg.MockLogger, req redis.DetachOciCacheUsersRequest) {
				mockClient.
					On("DetachOciCacheUsers", mock.Anything, mock.Anything).
					Return(redis.DetachOciCacheUsersResponse{}, fmt.Errorf("failed to detach cache users: OCI error"))

				mockLogger.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: true,
			errorMsg:    "failed to detach cache users",
		},
		{
			name:      "empty user list still calls API",
			userOCIDs: []string{},
			clusterID: "ocid1.rediscachecluster.oc1..cluster1",
			setupMocks: func(mockClient *MockRedisClusterClientInterface, mockLogger *testingPkg.MockLogger, req redis.DetachOciCacheUsersRequest) {
				resp := redis.DetachOciCacheUsersResponse{}

				mockClient.
					On("DetachOciCacheUsers", mock.Anything, mock.MatchedBy(func(r redis.DetachOciCacheUsersRequest) bool {
						if r.RedisClusterId == nil || *r.RedisClusterId != *req.RedisClusterId {
							return false
						}
						return len(r.DetachOciCacheUsersDetails.OciCacheUsers) == 0
					})).
					Return(resp, nil)

				mockLogger.On("Infof", mock.AnythingOfType("string"), mock.Anything).Maybe()
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLogger := testingPkg.SetupMockLogger()
			mockClient := &MockRedisClusterClientInterface{}

			req := redis.DetachOciCacheUsersRequest{
				RedisClusterId: common.String(tt.clusterID),
				DetachOciCacheUsersDetails: redis.DetachOciCacheUsersDetails{
					OciCacheUsers: tt.userOCIDs,
				},
			}

			tt.setupMocks(mockClient, mockLogger, req)

			resp, err := mockClient.DetachOciCacheUsers(context.Background(), req)

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
