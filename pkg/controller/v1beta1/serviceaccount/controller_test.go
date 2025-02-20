package serviceaccount

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk/option"
)

func stringPtr(s string) *string {
	return &s
}

// mockProjectServiceAccountService is a mock implementation of ProjectServiceAccountService
type mockProjectServiceAccountService struct {
	Options []option.RequestOption
}

func (m *mockProjectServiceAccountService) Create(ctx context.Context, projectID string, body openaisdk.ProjectServiceAccountCreateRequest, opts ...option.RequestOption) (*openaisdk.ProjectServiceAccountCreateResponse, error) {
	return &openaisdk.ProjectServiceAccountCreateResponse{
		ProjectServiceAccount: openaisdk.ProjectServiceAccount{
			ID:   "test-sa-id",
			Name: body.Name,
		},
		APIKey: &openaisdk.ProjectServiceAccountAPIKey{
			Value: "test-api-key-value",
		},
	}, nil
}

func (m *mockProjectServiceAccountService) Delete(ctx context.Context, projectID string, serviceAccountID string, opts ...option.RequestOption) (*openaisdk.ProjectServiceAccountDeleteResponse, error) {
	return &openaisdk.ProjectServiceAccountDeleteResponse{}, nil
}

func (m *mockProjectServiceAccountService) Get(ctx context.Context, projectID string, serviceAccountID string, opts ...option.RequestOption) (*openaisdk.ProjectServiceAccount, error) {
	return &openaisdk.ProjectServiceAccount{
		ID:   serviceAccountID,
		Name: "test-sa",
	}, nil
}

func (m *mockProjectServiceAccountService) List(ctx context.Context, projectID string, opts ...option.RequestOption) (*openaisdk.ProjectServiceAccountListResponse, error) {
	return &openaisdk.ProjectServiceAccountListResponse{}, nil
}

// mockOpenAIServer creates a test server that mocks OpenAI API responses
func mockOpenAIServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/organization/projects/") {
			if strings.Contains(r.URL.Path, "/service_accounts") {
				switch r.Method {
				case http.MethodPost:
					// Handle service account creation
					response := openaisdk.ProjectServiceAccountCreateResponse{
						ProjectServiceAccount: openaisdk.ProjectServiceAccount{
							Object:    "organization.project.service_account",
							ID:        "test-sa-id",
							Name:      "test-sa",
							Role:      "member",
							CreatedAt: time.Now().Unix(),
						},
						APIKey: &openaisdk.ProjectServiceAccountAPIKey{
							Object:    "organization.project.service_account.api_key",
							Value:     "test-api-key-value",
							Name:      "test-sa",
							CreatedAt: time.Now().Unix(),
							ID:        "test-api-key-id",
						},
					}
					if err := json.NewEncoder(w).Encode(response); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
				case http.MethodDelete:
					// Handle service account deletion
					response := openaisdk.ProjectServiceAccountDeleteResponse{
						Object:  "organization.project.service_account",
						ID:      "test-sa-id",
						Deleted: true,
					}
					if err := json.NewEncoder(w).Encode(response); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
				}
			}
		}
	}))
}

func TestServiceAccountReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	mockServer := mockOpenAIServer()
	defer mockServer.Close()

	testProject := &v1beta1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-project",
			Namespace: "default",
		},
		Status: v1beta1.ProjectStatus{
			ProjectID: "test-project-id",
		},
		Spec: v1beta1.ProjectSpec{
			OrganizationRef: v1beta1.CrossReference{
				Name: "test-org",
			},
		},
	}

	testOrg := &v1beta1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: v1beta1.OrganizationSpec{
			SecretRef: v1beta1.SecretReference{
				Name:      "test-org-secret",
				Namespace: "default",
				Key:       "api-key",
			},
			Vendor:         stringPtr("openai"),
			OrganizationID: "test-org-id",
		},
	}

	testSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-org-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"api-key": []byte("test-api-key"),
		},
	}

	tests := []struct {
		name           string
		serviceAccount *v1beta1.ServiceAccount
		setupFunc      func(client.Client)
		expectedResult ctrl.Result
		expectedError  bool
		verifyFunc     func(t *testing.T, client client.Client)
	}{
		{
			name:           "service account not found",
			serviceAccount: nil,
			expectedResult: ctrl.Result{},
			expectedError:  false,
			verifyFunc: func(t *testing.T, client client.Client) {
				// No verification needed as the service account is not found
			},
		},
		{
			name: "create service account with API key",
			serviceAccount: &v1beta1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
				},
				Spec: v1beta1.ServiceAccountSpec{
					Name: "test-sa",
					ProjectRef: v1beta1.CrossReference{
						Name:      "test-project",
						Namespace: "default",
					},
				},
			},
			setupFunc: func(c client.Client) {
				require.NoError(t, c.Create(context.Background(), testProject))
				require.NoError(t, c.Create(context.Background(), testOrg))
				require.NoError(t, c.Create(context.Background(), testSecret))

				// Create a clean copy of the service account without status
				sa := &v1beta1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sa",
						Namespace: "default",
					},
					Spec: v1beta1.ServiceAccountSpec{
						Name: "test-sa",
						ProjectRef: v1beta1.CrossReference{
							Name:      "test-project",
							Namespace: "default",
						},
					},
				}
				require.NoError(t, c.Create(context.Background(), sa))
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
			verifyFunc: func(t *testing.T, c client.Client) {
				// Verify service account was updated with finalizer and status
				sa := &v1beta1.ServiceAccount{}
				err := c.Get(context.Background(), types.NamespacedName{
					Name:      "test-sa",
					Namespace: "default",
				}, sa)
				require.NoError(t, err)

				// Verify finalizer was added
				assert.Contains(t, sa.Finalizers, finalizerName)

				// Verify service account ID was set
				assert.Equal(t, "test-sa-id", sa.Status.ServiceAccountID)

				// Verify API key secret reference was set
				if assert.NotNil(t, sa.Status.APIKeySecretRef) {
					assert.Equal(t, "test-sa-apikey", sa.Status.APIKeySecretRef.Name)
					assert.Equal(t, "default", sa.Status.APIKeySecretRef.Namespace)
				}

				// Verify API key secret was created
				secret := &corev1.Secret{}
				err = c.Get(context.Background(), types.NamespacedName{
					Name:      "test-sa-apikey",
					Namespace: "default",
				}, secret)
				if assert.NoError(t, err) {
					assert.Equal(t, "test-api-key-value", string(secret.Data["api-key"]))
				}
			},
		},
		{
			name: "missing project reference",
			serviceAccount: &v1beta1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sa",
					Namespace: "default",
				},
				Spec: v1beta1.ServiceAccountSpec{
					Name: "test-sa",
					ProjectRef: v1beta1.CrossReference{
						Name: "non-existent-project",
					},
				},
			},
			setupFunc: func(c client.Client) {
				// Create a clean copy of the service account without status
				sa := &v1beta1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sa",
						Namespace: "default",
					},
					Spec: v1beta1.ServiceAccountSpec{
						Name: "test-sa",
						ProjectRef: v1beta1.CrossReference{
							Name: "non-existent-project",
						},
					},
				}
				require.NoError(t, c.Create(context.Background(), sa))
			},
			expectedResult: ctrl.Result{},
			expectedError:  true,
			verifyFunc: func(t *testing.T, c client.Client) {
				// Verify no secret was created
				secret := &corev1.Secret{}
				err := c.Get(context.Background(), types.NamespacedName{
					Name:      "test-sa-apikey",
					Namespace: "default",
				}, secret)
				assert.Error(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new fake client for each test case with status subresource support
			builder := cfake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&v1beta1.ServiceAccount{})

			fakeClient := builder.Build()

			reconciler := &ServiceAccountReconciler{
				Client:    fakeClient,
				Clientset: kfake.NewSimpleClientset(),
				Log:       logr.Discard(),
				Scheme:    scheme,
				Recorder:  record.NewFakeRecorder(100),
				OpenAIClientFactory: func(apiKey string, baseURL string) *openaisdk.Client {
					return openaisdk.NewClient(
						option.WithAPIKey(apiKey),
						option.WithBaseURL(mockServer.URL),
					)
				},
			}

			if tt.setupFunc != nil {
				tt.setupFunc(fakeClient)
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-sa",
					Namespace: "default",
				},
			}

			res, err := reconciler.Reconcile(context.Background(), req)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, res)

			if tt.verifyFunc != nil {
				tt.verifyFunc(t, fakeClient)
			}
		})
	}
}

func TestServiceAccountReconciler_getProjectID(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)

	tests := []struct {
		name           string
		serviceAccount *v1beta1.ServiceAccount
		project        *v1beta1.Project
		expectedError  bool
		expectedID     string
	}{
		{
			name: "project exists",
			serviceAccount: &v1beta1.ServiceAccount{
				Spec: v1beta1.ServiceAccountSpec{
					ProjectRef: v1beta1.CrossReference{
						Name:      "test-project",
						Namespace: "default",
					},
				},
			},
			project: &v1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-project",
					Namespace: "default",
				},
				Status: v1beta1.ProjectStatus{
					ProjectID: "test-project-id",
				},
			},
			expectedError: false,
			expectedID:    "test-project-id",
		},
		{
			name: "project does not exist",
			serviceAccount: &v1beta1.ServiceAccount{
				Spec: v1beta1.ServiceAccountSpec{
					ProjectRef: v1beta1.CrossReference{
						Name: "non-existent-project",
					},
				},
			},
			project:       nil,
			expectedError: true,
			expectedID:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := cfake.NewClientBuilder().WithScheme(scheme).Build()
			reconciler := &ServiceAccountReconciler{
				Client:   fakeClient,
				Log:      logr.Discard(),
				Scheme:   scheme,
				Recorder: record.NewFakeRecorder(100),
			}

			if tt.project != nil {
				err := fakeClient.Create(context.Background(), tt.project)
				require.NoError(t, err)
			}

			projectID, err := reconciler.getProjectID(context.Background(), tt.serviceAccount)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedID, projectID)
		})
	}
}
