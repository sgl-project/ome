package aws

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go-v2/credentials/endpointcreds"
	"github.com/aws/aws-sdk-go-v2/credentials/processcreds"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

// Factory creates AWS credentials
type Factory struct {
	logger logging.Interface
}

// NewFactory creates a new AWS auth factory
func NewFactory(logger logging.Interface) *Factory {
	return &Factory{
		logger: logger,
	}
}

// Create creates AWS credentials based on config
func (f *Factory) Create(ctx context.Context, config auth.Config) (auth.Credentials, error) {
	if config.Provider != auth.ProviderAWS {
		return nil, fmt.Errorf("invalid provider: expected %s, got %s", auth.ProviderAWS, config.Provider)
	}

	var credProvider aws.CredentialsProvider
	var err error

	switch config.AuthType {
	case auth.AWSAccessKey:
		credProvider, err = f.createAccessKeyProvider(config)
	case auth.AWSAssumeRole:
		credProvider, err = f.createAssumeRoleProvider(ctx, config)
	case auth.AWSInstanceProfile:
		credProvider, err = f.createInstanceProfileProvider(ctx, config)
	case auth.AWSWebIdentity:
		credProvider, err = f.createWebIdentityProvider(ctx, config)
	case auth.AWSECSTaskRole:
		credProvider, err = f.createECSTaskRoleProvider(ctx, config)
	case auth.AWSProcess:
		credProvider, err = f.createProcessProvider(config)
	case auth.AWSDefault:
		credProvider, err = f.createDefaultProvider(ctx, config)
	default:
		return nil, fmt.Errorf("unsupported AWS auth type: %s", config.AuthType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create AWS credentials provider: %w", err)
	}

	return &AWSCredentials{
		credProvider: credProvider,
		authType:     config.AuthType,
		region:       config.Region,
		logger:       f.logger,
	}, nil
}

// SupportedAuthTypes returns supported AWS auth types
func (f *Factory) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{
		auth.AWSAccessKey,
		auth.AWSAssumeRole,
		auth.AWSInstanceProfile,
		auth.AWSWebIdentity,
		auth.AWSECSTaskRole,
		auth.AWSProcess,
		auth.AWSDefault,
	}
}

// createAccessKeyProvider creates an access key credentials provider
func (f *Factory) createAccessKeyProvider(config auth.Config) (aws.CredentialsProvider, error) {
	// Extract access key config
	akConfig := AccessKeyConfig{}

	if config.Extra != nil {
		if ak, ok := config.Extra["access_key"].(map[string]interface{}); ok {
			if accessKeyID, ok := ak["access_key_id"].(string); ok {
				akConfig.AccessKeyID = accessKeyID
			}
			if secretAccessKey, ok := ak["secret_access_key"].(string); ok {
				akConfig.SecretAccessKey = secretAccessKey
			}
			if sessionToken, ok := ak["session_token"].(string); ok {
				akConfig.SessionToken = sessionToken
			}
		}
	}

	// Check environment variables
	if akConfig.AccessKeyID == "" {
		akConfig.AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	if akConfig.SecretAccessKey == "" {
		akConfig.SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}
	if akConfig.SessionToken == "" {
		akConfig.SessionToken = os.Getenv("AWS_SESSION_TOKEN")
	}

	// Validate
	if err := akConfig.Validate(); err != nil {
		return nil, err
	}

	return credentials.NewStaticCredentialsProvider(
		akConfig.AccessKeyID,
		akConfig.SecretAccessKey,
		akConfig.SessionToken,
	), nil
}

// createAssumeRoleProvider creates an assume role credentials provider
func (f *Factory) createAssumeRoleProvider(ctx context.Context, config auth.Config) (aws.CredentialsProvider, error) {
	// Extract assume role config
	arConfig := AssumeRoleConfig{}

	if config.Extra != nil {
		if ar, ok := config.Extra["assume_role"].(map[string]interface{}); ok {
			if roleARN, ok := ar["role_arn"].(string); ok {
				arConfig.RoleARN = roleARN
			}
			if roleSessionName, ok := ar["role_session_name"].(string); ok {
				arConfig.RoleSessionName = roleSessionName
			}
			if externalID, ok := ar["external_id"].(string); ok {
				arConfig.ExternalID = externalID
			}
		}
	}

	// Check environment variables
	if arConfig.RoleARN == "" {
		arConfig.RoleARN = os.Getenv("AWS_ROLE_ARN")
	}
	if arConfig.RoleSessionName == "" {
		arConfig.RoleSessionName = os.Getenv("AWS_ROLE_SESSION_NAME")
		if arConfig.RoleSessionName == "" {
			arConfig.RoleSessionName = "ome-storage-session"
		}
	}

	// Validate
	if err := arConfig.Validate(); err != nil {
		return nil, err
	}

	// Load base config with region if specified
	configOpts := []func(*awsconfig.LoadOptions) error{}
	if config.Region != "" {
		configOpts = append(configOpts, awsconfig.WithRegion(config.Region))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create STS client
	stsClient := sts.NewFromConfig(cfg)

	// Create assume role provider
	provider := stscreds.NewAssumeRoleProvider(stsClient, arConfig.RoleARN, func(o *stscreds.AssumeRoleOptions) {
		if arConfig.RoleSessionName != "" {
			o.RoleSessionName = arConfig.RoleSessionName
		}
		if arConfig.ExternalID != "" {
			o.ExternalID = &arConfig.ExternalID
		}
		if arConfig.Duration > 0 {
			o.Duration = arConfig.Duration
		}
	})

	return provider, nil
}

// createInstanceProfileProvider creates an EC2 instance profile credentials provider
func (f *Factory) createInstanceProfileProvider(ctx context.Context, config auth.Config) (aws.CredentialsProvider, error) {
	// Create EC2 role credentials provider
	provider := ec2rolecreds.New()

	return provider, nil
}

// createDefaultProvider creates a default credentials provider chain
func (f *Factory) createDefaultProvider(ctx context.Context, config auth.Config) (aws.CredentialsProvider, error) {
	// Load default config with region if specified
	configOpts := []func(*awsconfig.LoadOptions) error{}
	if config.Region != "" {
		configOpts = append(configOpts, awsconfig.WithRegion(config.Region))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return cfg.Credentials, nil
}

// createWebIdentityProvider creates a web identity credentials provider
func (f *Factory) createWebIdentityProvider(ctx context.Context, config auth.Config) (aws.CredentialsProvider, error) {
	// Extract web identity config
	wiConfig := WebIdentityConfig{}

	if config.Extra != nil {
		if wi, ok := config.Extra["web_identity"].(map[string]interface{}); ok {
			if roleArn, ok := wi["role_arn"].(string); ok {
				wiConfig.RoleARN = roleArn
			}
			if tokenFile, ok := wi["token_file"].(string); ok {
				wiConfig.TokenFile = tokenFile
			}
			if sessionName, ok := wi["role_session_name"].(string); ok {
				wiConfig.RoleSessionName = sessionName
			}
		}
	}

	// Check environment variables as fallback
	if wiConfig.RoleARN == "" {
		wiConfig.RoleARN = os.Getenv("AWS_ROLE_ARN")
	}
	if wiConfig.TokenFile == "" {
		wiConfig.TokenFile = os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	}
	if wiConfig.RoleSessionName == "" {
		wiConfig.RoleSessionName = os.Getenv("AWS_ROLE_SESSION_NAME")
		if wiConfig.RoleSessionName == "" {
			wiConfig.RoleSessionName = fmt.Sprintf("aws-web-identity-%d", time.Now().Unix())
		}
	}

	// Validate
	if err := wiConfig.Validate(); err != nil {
		return nil, err
	}

	// Load base config with region if specified
	configOpts := []func(*awsconfig.LoadOptions) error{}
	if config.Region != "" {
		configOpts = append(configOpts, awsconfig.WithRegion(config.Region))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create STS client
	stsClient := sts.NewFromConfig(cfg)

	// Create web identity role provider
	provider := stscreds.NewWebIdentityRoleProvider(
		stsClient,
		wiConfig.RoleARN,
		stscreds.IdentityTokenFile(wiConfig.TokenFile),
		func(o *stscreds.WebIdentityRoleOptions) {
			o.RoleSessionName = wiConfig.RoleSessionName
		},
	)

	return provider, nil
}

// createECSTaskRoleProvider creates an ECS task role credentials provider
func (f *Factory) createECSTaskRoleProvider(ctx context.Context, config auth.Config) (aws.CredentialsProvider, error) {
	// Extract ECS task role config
	ecsConfig := ECSTaskRoleConfig{}

	if config.Extra != nil {
		if ecs, ok := config.Extra["ecs_task_role"].(map[string]interface{}); ok {
			if relativeURI, ok := ecs["relative_uri"].(string); ok {
				ecsConfig.RelativeURI = relativeURI
			}
			if fullURI, ok := ecs["full_uri"].(string); ok {
				ecsConfig.FullURI = fullURI
			}
			if authToken, ok := ecs["authorization_token"].(string); ok {
				ecsConfig.AuthorizationToken = authToken
			}
		}
	}

	// Check environment variables as fallback
	if ecsConfig.RelativeURI == "" {
		ecsConfig.RelativeURI = os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI")
	}
	if ecsConfig.FullURI == "" {
		ecsConfig.FullURI = os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI")
	}
	if ecsConfig.AuthorizationToken == "" {
		ecsConfig.AuthorizationToken = os.Getenv("AWS_CONTAINER_AUTHORIZATION_TOKEN")
	}

	// Validate
	if err := ecsConfig.Validate(); err != nil {
		return nil, err
	}

	// Create endpoint credentials provider based on whether we have relative or full URI
	if ecsConfig.FullURI != "" {
		// Use full URI endpoint
		options := []func(*endpointcreds.Options){}
		if ecsConfig.AuthorizationToken != "" {
			options = append(options, func(o *endpointcreds.Options) {
				o.AuthorizationToken = ecsConfig.AuthorizationToken
			})
		}
		return endpointcreds.New(ecsConfig.FullURI, options...), nil
	}

	// Use relative URI with the ECS credentials endpoint base
	ecsEndpoint := fmt.Sprintf("http://169.254.170.2%s", ecsConfig.RelativeURI)
	return endpointcreds.New(ecsEndpoint), nil
}

// createProcessProvider creates a process credentials provider
func (f *Factory) createProcessProvider(config auth.Config) (aws.CredentialsProvider, error) {
	// Extract process config
	procConfig := ProcessConfig{}

	if config.Extra != nil {
		if proc, ok := config.Extra["process"].(map[string]interface{}); ok {
			if command, ok := proc["command"].(string); ok {
				procConfig.Command = command
			}
			// Handle timeout - could be string, int64, or float64 from JSON/YAML
			if timeoutVal, ok := proc["timeout"]; ok {
				switch v := timeoutVal.(type) {
				case string:
					if duration, err := time.ParseDuration(v); err == nil {
						procConfig.Timeout = duration
					}
				case float64:
					procConfig.Timeout = time.Duration(v) * time.Second
				case int64:
					procConfig.Timeout = time.Duration(v) * time.Second
				case int:
					procConfig.Timeout = time.Duration(v) * time.Second
				}
			}
		}
	}

	// Check environment variable as fallback
	if procConfig.Command == "" {
		procConfig.Command = os.Getenv("AWS_CREDENTIAL_PROCESS")
	}

	// Set default timeout if not specified
	if procConfig.Timeout == 0 {
		procConfig.Timeout = 1 * time.Minute
	}

	// Validate
	if err := procConfig.Validate(); err != nil {
		return nil, err
	}

	// Create process credentials provider with timeout if specified
	options := []func(*processcreds.Options){}
	if procConfig.Timeout > 0 {
		options = append(options, func(o *processcreds.Options) {
			o.Timeout = procConfig.Timeout
		})
	}

	return processcreds.NewProvider(procConfig.Command, options...), nil
}
