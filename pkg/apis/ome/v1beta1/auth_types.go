package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AuthMethod defines the authentication method to use.
// +kubebuilder:validation:Enum=None;Bearer;ApiKey;Basic;JWT;ClientCertificate;OAuth2
type AuthMethod string

const (
	AuthMethodNone              AuthMethod = "None"
	AuthMethodBearer            AuthMethod = "Bearer"
	AuthMethodApiKey            AuthMethod = "ApiKey"
	AuthMethodBasic             AuthMethod = "Basic"
	AuthMethodJWT               AuthMethod = "JWT"
	AuthMethodClientCertificate AuthMethod = "ClientCertificate"
	AuthMethodOAuth2            AuthMethod = "OAuth2"
)

// CredentialRef provides a reference to a secret containing authentication credentials.
type CredentialRef struct {
	// SecretRef references a Kubernetes secret containing the credential.
	// +optional
	SecretRef *corev1.SecretKeySelector `json:"secretRef,omitempty"`

	// Value contains the credential value directly (not recommended for sensitive data).
	// +optional
	Value string `json:"value,omitempty"`

	// HeaderName specifies the header name for API key authentication.
	// +optional
	HeaderName string `json:"headerName,omitempty"`
}

// AuthConfig provides unified authentication configuration for all components.
type AuthConfig struct {
	// Method defines the authentication method to use.
	// +kubebuilder:validation:Required
	Method AuthMethod `json:"method"`

	// Token provides the authentication token (Bearer, API Key).
	// +optional
	Token *CredentialRef `json:"token,omitempty"`

	// Basic provides basic authentication credentials.
	// +optional
	Basic *BasicCredentials `json:"basic,omitempty"`

	// JWT provides JWT authentication configuration.
	// +optional
	JWT *JWTCredentials `json:"jwt,omitempty"`

	// ClientCert provides client certificate authentication.
	// +optional
	ClientCert *ClientCertCredentials `json:"clientCert,omitempty"`

	// OAuth2 provides OAuth2 authentication configuration.
	// +optional
	OAuth2 *OAuth2Credentials `json:"oAuth2,omitempty"`

	// Timeout defines the authentication request timeout.
	// +kubebuilder:default="30s"
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

// BasicCredentials defines basic authentication credentials.
type BasicCredentials struct {
	// Username for basic authentication.
	// +kubebuilder:validation:Required
	Username string `json:"username"`

	// Password references the password secret.
	// +kubebuilder:validation:Required
	Password CredentialRef `json:"password"`
}

// JWTCredentials defines JWT authentication credentials.
type JWTCredentials struct {
	// SigningKey references the JWT signing key secret.
	// +kubebuilder:validation:Required
	SigningKey CredentialRef `json:"signingKey"`

	// Algorithm defines the JWT signing algorithm.
	// +kubebuilder:validation:Enum=HS256;HS384;HS512;RS256;RS384;RS512;ES256;ES384;ES512
	// +kubebuilder:default=RS256
	// +optional
	Algorithm string `json:"algorithm,omitempty"`

	// Issuer defines the expected JWT issuer.
	// +optional
	Issuer string `json:"issuer,omitempty"`

	// Audience defines the expected JWT audience.
	// +optional
	Audience string `json:"audience,omitempty"`

	// ExpirationTolerance defines tolerance for token expiration.
	// +kubebuilder:default="30s"
	// +optional
	ExpirationTolerance *metav1.Duration `json:"expirationTolerance,omitempty"`
}

// ClientCertCredentials defines client certificate authentication.
type ClientCertCredentials struct {
	// CertificateRef references the client certificate secret.
	// +kubebuilder:validation:Required
	CertificateRef CredentialRef `json:"certificateRef"`

	// PrivateKeyRef references the private key secret.
	// +kubebuilder:validation:Required
	PrivateKeyRef CredentialRef `json:"privateKeyRef"`

	// CARef references the CA certificate secret for verification.
	// +optional
	CARef *CredentialRef `json:"caRef,omitempty"`

	// VerifyServerCert controls whether to verify the server certificate.
	// +kubebuilder:default=true
	// +optional
	VerifyServerCert *bool `json:"verifyServerCert,omitempty"`
}

// OAuth2Credentials defines OAuth2 authentication credentials.
type OAuth2Credentials struct {
	// ClientID for OAuth2 authentication.
	// +kubebuilder:validation:Required
	ClientID string `json:"clientID"`

	// ClientSecret references the OAuth2 client secret.
	// +kubebuilder:validation:Required
	ClientSecret CredentialRef `json:"clientSecret"`

	// TokenURL is the OAuth2 token endpoint.
	// +kubebuilder:validation:Required
	TokenURL string `json:"tokenURL"`

	// Scopes define the OAuth2 scopes to request.
	// +optional
	// +listType=set
	Scopes []string `json:"scopes,omitempty"`
}
