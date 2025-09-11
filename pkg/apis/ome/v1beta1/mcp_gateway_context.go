package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MemoryBucket defines an organized memory storage bucket.
type MemoryBucket struct {
	// Name is the bucket identifier.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Description provides a human-readable description of the bucket.
	// +optional
	Description string `json:"description,omitempty"`

	// TTL defines the time-to-live for entries in this bucket.
	// +optional
	TTL *metav1.Duration `json:"ttl,omitempty"`

	// MaxEntries defines the maximum number of entries in this bucket.
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxEntries *int32 `json:"maxEntries,omitempty"`

	// AccessControl defines who can access this bucket.
	// +optional
	AccessControl *BucketAccessControl `json:"accessControl,omitempty"`
}

// BucketAccessControl defines access control for memory buckets.
type BucketAccessControl struct {
	// AllowedUsers defines users that can access this bucket.
	// +optional
	// +listType=set
	AllowedUsers []string `json:"allowedUsers,omitempty"`

	// AllowedTenants defines tenants that can access this bucket.
	// +optional
	// +listType=set
	AllowedTenants []string `json:"allowedTenants,omitempty"`
}

// MemoryBucketConfig defines context and memory management with bucket support.
type MemoryBucketConfig struct {
	// Storage defines the storage backend for memory and context.
	// +optional
	Storage *MemoryStorageConfig `json:"storage,omitempty"`

	// Buckets define organized memory storage buckets.
	// +optional
	// +listType=atomic
	Buckets []MemoryBucket `json:"buckets,omitempty"`

	// DefaultTTL defines the default time-to-live for memory entries.
	// +kubebuilder:default="1h"
	// +optional
	DefaultTTL *metav1.Duration `json:"defaultTTL,omitempty"`

	// MaxSize defines the maximum memory storage size.
	// +optional
	MaxSize *resource.Quantity `json:"maxSize,omitempty"`
}

// SessionContextConfig defines context storage and session management policies.
type SessionContextConfig struct {
	// Storage defines where and how context is persisted.
	// +optional
	Storage *ContextStorageConfig `json:"storage,omitempty"`

	// Sessions define session management policies.
	// +optional
	Sessions *SessionConfig `json:"sessions,omitempty"`

	// Isolation defines context isolation policies for multi-tenancy.
	// +optional
	Isolation *ContextIsolationConfig `json:"isolation,omitempty"`

	// Persistence defines context persistence policies.
	// +optional
	Persistence *ContextPersistenceConfig `json:"persistence,omitempty"`
}

// ContextStorageConfig defines context storage configuration.
type ContextStorageConfig struct {
	// Type defines the storage backend type.
	// +kubebuilder:validation:Enum=Memory;Redis;Database;File
	// +kubebuilder:default=Memory
	// +optional
	Type ContextStorageType `json:"type,omitempty"`

	// Memory defines in-memory storage configuration.
	// +optional
	Memory *MemoryStorageConfig `json:"memory,omitempty"`

	// Redis defines Redis storage configuration.
	// +optional
	Redis *RedisStorageConfig `json:"redis,omitempty"`

	// Database defines database storage configuration.
	// +optional
	Database *DatabaseStorageConfig `json:"database,omitempty"`

	// File defines file-based storage configuration.
	// +optional
	File *FileStorageConfig `json:"file,omitempty"`
}

// ContextStorageType defines context storage types.
type ContextStorageType string

const (
	ContextStorageTypeMemory   ContextStorageType = "Memory"
	ContextStorageTypeRedis    ContextStorageType = "Redis"
	ContextStorageTypeDatabase ContextStorageType = "Database"
	ContextStorageTypeFile     ContextStorageType = "File"
)

// MemoryStorageConfig defines in-memory storage configuration.
type MemoryStorageConfig struct {
	// MaxSize defines maximum memory usage for context storage.
	// +optional
	MaxSize *resource.Quantity `json:"maxSize,omitempty"`

	// EvictionPolicy defines how to evict contexts when memory is full.
	// +kubebuilder:validation:Enum=LRU;LFU;TTL
	// +kubebuilder:default=LRU
	// +optional
	EvictionPolicy MemoryEvictionPolicy `json:"evictionPolicy,omitempty"`
}

// MemoryEvictionPolicy defines memory eviction policies.
type MemoryEvictionPolicy string

const (
	MemoryEvictionPolicyLRU MemoryEvictionPolicy = "LRU"
	MemoryEvictionPolicyLFU MemoryEvictionPolicy = "LFU"
	MemoryEvictionPolicyTTL MemoryEvictionPolicy = "TTL"
)

// RedisStorageConfig defines Redis storage configuration.
type RedisStorageConfig struct {
	// Address specifies the Redis server address.
	// +kubebuilder:validation:Required
	Address string `json:"address"`

	// Database specifies the Redis database number.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	Database *int32 `json:"database,omitempty"`

	// Auth defines Redis authentication.
	// +optional
	Auth *RedisAuthConfig `json:"auth,omitempty"`

	// TLS defines Redis TLS configuration.
	// +optional
	TLS *RedisTLSConfig `json:"tls,omitempty"`

	// Pool defines Redis connection pooling.
	// +optional
	Pool *RedisPoolConfig `json:"pool,omitempty"`
}

// RedisAuthConfig defines Redis authentication.
type RedisAuthConfig struct {
	// PasswordSecretRef references a secret containing the Redis password.
	// +optional
	PasswordSecretRef *corev1.SecretKeySelector `json:"passwordSecretRef,omitempty"`

	// Username specifies the Redis username.
	// +optional
	Username string `json:"username,omitempty"`
}

// RedisTLSConfig defines Redis TLS configuration.
type RedisTLSConfig struct {
	// Enabled controls whether TLS is used.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// InsecureSkipVerify skips certificate verification.
	// +kubebuilder:default=false
	// +optional
	InsecureSkipVerify *bool `json:"insecureSkipVerify,omitempty"`

	// CertSecretRef references a secret containing TLS certificates.
	// +optional
	CertSecretRef *corev1.SecretKeySelector `json:"certSecretRef,omitempty"`
}

// RedisPoolConfig defines Redis connection pooling.
type RedisPoolConfig struct {
	// MinIdleConnections defines the minimum number of idle connections.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=2
	// +optional
	MinIdleConnections *int32 `json:"minIdleConnections,omitempty"`

	// MaxConnections defines the maximum number of connections.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	// +optional
	MaxConnections *int32 `json:"maxConnections,omitempty"`
}

// DatabaseStorageConfig defines database storage configuration.
type DatabaseStorageConfig struct {
	// Type defines the database type.
	// +kubebuilder:validation:Enum=PostgreSQL;MySQL;SQLite
	// +kubebuilder:default=PostgreSQL
	// +optional
	Type DatabaseType `json:"type,omitempty"`

	// ConnectionString defines the database connection string.
	// +kubebuilder:validation:Required
	ConnectionString string `json:"connectionString"`

	// ConnectionSecretRef references a secret containing database credentials.
	// +optional
	ConnectionSecretRef *corev1.SecretKeySelector `json:"connectionSecretRef,omitempty"`

	// MaxConnections defines the maximum number of database connections.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=5
	// +optional
	MaxConnections *int32 `json:"maxConnections,omitempty"`
}

// DatabaseType defines supported database types.
type DatabaseType string

const (
	DatabaseTypePostgreSQL DatabaseType = "PostgreSQL"
	DatabaseTypeMySQL      DatabaseType = "MySQL"
	DatabaseTypeSQLite     DatabaseType = "SQLite"
)

// FileStorageConfig defines file-based storage configuration.
type FileStorageConfig struct {
	// Path defines the file system path for context storage.
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// VolumeSource defines the volume source for persistent storage.
	// +optional
	VolumeSource *corev1.VolumeSource `json:"volumeSource,omitempty"`
}

// SessionConfig defines session management policies.
type SessionConfig struct {
	// DefaultTTL defines the default session time-to-live.
	// +kubebuilder:default="1h"
	// +optional
	DefaultTTL *metav1.Duration `json:"defaultTTL,omitempty"`

	// MaxTTL defines the maximum allowed session time-to-live.
	// +kubebuilder:default="24h"
	// +optional
	MaxTTL *metav1.Duration `json:"maxTTL,omitempty"`

	// IdleTimeout defines when to expire idle sessions.
	// +kubebuilder:default="15m"
	// +optional
	IdleTimeout *metav1.Duration `json:"idleTimeout,omitempty"`

	// MaxConcurrentSessions defines the maximum concurrent sessions per user.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	// +optional
	MaxConcurrentSessions *int32 `json:"maxConcurrentSessions,omitempty"`
}

// ContextIsolationConfig defines context isolation policies for multi-tenancy.
type ContextIsolationConfig struct {
	// Level defines the isolation level.
	// +kubebuilder:validation:Enum=None;User;Tenant;Namespace
	// +kubebuilder:default=User
	// +optional
	Level ContextIsolationLevel `json:"level,omitempty"`

	// TenantKey defines how to extract tenant information from requests.
	// +optional
	TenantKey *TenantKeyConfig `json:"tenantKey,omitempty"`

	// Encryption defines whether to encrypt context data.
	// +optional
	Encryption *ContextEncryptionConfig `json:"encryption,omitempty"`
}

// ContextIsolationLevel defines context isolation levels.
type ContextIsolationLevel string

const (
	ContextIsolationLevelNone      ContextIsolationLevel = "None"
	ContextIsolationLevelUser      ContextIsolationLevel = "User"
	ContextIsolationLevelTenant    ContextIsolationLevel = "Tenant"
	ContextIsolationLevelNamespace ContextIsolationLevel = "Namespace"
)

// TenantKeyConfig defines how to extract tenant information.
type TenantKeyConfig struct {
	// Source defines where to find tenant information.
	// +kubebuilder:validation:Enum=Header;JWT;Certificate;Query
	// +kubebuilder:default=Header
	// +optional
	Source TenantKeySource `json:"source,omitempty"`

	// Name specifies the header/query parameter/JWT claim name.
	// +optional
	Name string `json:"name,omitempty"`
}

// TenantKeySource defines tenant key sources.
type TenantKeySource string

const (
	TenantKeySourceHeader      TenantKeySource = "Header"
	TenantKeySourceJWT         TenantKeySource = "JWT"
	TenantKeySourceCertificate TenantKeySource = "Certificate"
	TenantKeySourceQuery       TenantKeySource = "Query"
)

// ContextEncryptionConfig defines context encryption configuration.
type ContextEncryptionConfig struct {
	// Enabled controls whether context encryption is active.
	// +kubebuilder:default=false
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Algorithm defines the encryption algorithm.
	// +kubebuilder:validation:Enum=AES-256-GCM;ChaCha20-Poly1305
	// +kubebuilder:default=AES-256-GCM
	// +optional
	Algorithm EncryptionAlgorithm `json:"algorithm,omitempty"`

	// KeySecretRef references a secret containing encryption keys.
	// +optional
	KeySecretRef *corev1.SecretKeySelector `json:"keySecretRef,omitempty"`
}

// EncryptionAlgorithm defines supported encryption algorithms.
type EncryptionAlgorithm string

const (
	EncryptionAlgorithmAES256GCM        EncryptionAlgorithm = "AES-256-GCM"
	EncryptionAlgorithmChaCha20Poly1305 EncryptionAlgorithm = "ChaCha20-Poly1305"
)

// ContextPersistenceConfig defines context persistence policies.
type ContextPersistenceConfig struct {
	// Strategy defines the persistence strategy.
	// +kubebuilder:validation:Enum=None;Session;Workflow;Permanent
	// +kubebuilder:default=Session
	// +optional
	Strategy ContextPersistenceStrategy `json:"strategy,omitempty"`

	// WorkflowTTL defines how long to persist workflow contexts.
	// +kubebuilder:default="7d"
	// +optional
	WorkflowTTL *metav1.Duration `json:"workflowTTL,omitempty"`

	// MaxContextSize defines the maximum size of a persisted context.
	// +optional
	MaxContextSize *resource.Quantity `json:"maxContextSize,omitempty"`
}

// ContextPersistenceStrategy defines context persistence strategies.
type ContextPersistenceStrategy string

const (
	ContextPersistenceStrategyNone      ContextPersistenceStrategy = "None"
	ContextPersistenceStrategySession   ContextPersistenceStrategy = "Session"
	ContextPersistenceStrategyWorkflow  ContextPersistenceStrategy = "Workflow"
	ContextPersistenceStrategyPermanent ContextPersistenceStrategy = "Permanent"
)

// MemoryBucketStatus provides status of a memory bucket.
type MemoryBucketStatus struct {
	// Name is the bucket identifier.
	Name string `json:"name"`

	// EntryCount is the current number of entries in the bucket.
	// +optional
	EntryCount int32 `json:"entryCount,omitempty"`

	// SizeBytes is the current size of the bucket in bytes.
	// +optional
	SizeBytes int64 `json:"sizeBytes,omitempty"`

	// LastAccessed is the timestamp of the last access.
	// +optional
	LastAccessed *metav1.Time `json:"lastAccessed,omitempty"`
}