package oci

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// GetPresignedURL generates a presigned URL for temporary access to an object
func (p *OCIProvider) GetPresignedURL(ctx context.Context, uri string, expiry time.Duration) (string, error) {
	// Parse the URI
	ociURI, err := parseOCIURI(uri, p.namespace, p.bucket)
	if err != nil {
		return "", fmt.Errorf("failed to parse URI: %w", err)
	}

	// Create a pre-authenticated request (PAR) for the object
	// This is OCI's equivalent of presigned URLs
	createPARRequest := objectstorage.CreatePreauthenticatedRequestRequest{
		NamespaceName: &ociURI.Namespace,
		BucketName:    &ociURI.Bucket,
		CreatePreauthenticatedRequestDetails: objectstorage.CreatePreauthenticatedRequestDetails{
			Name:       common.String(fmt.Sprintf("temp-access-%s-%d", ociURI.Object, time.Now().Unix())),
			ObjectName: &ociURI.Object,
			AccessType: objectstorage.CreatePreauthenticatedRequestDetailsAccessTypeObjectread,
			TimeExpires: &common.SDKTime{
				Time: time.Now().Add(expiry),
			},
		},
	}

	// Create the PAR
	response, err := p.client.CreatePreauthenticatedRequest(ctx, createPARRequest)
	if err != nil {
		return "", fmt.Errorf("failed to create pre-authenticated request: %w", err)
	}

	// Build the full URL
	// The PAR access URI is relative, we need to build the full URL
	region := p.region
	if region == "" {
		// Default to us-phoenix-1 if no region configured
		region = "us-phoenix-1"
		p.logger.Warn("No region configured, using default us-phoenix-1")
	}

	// Construct the base URL for the object storage service
	baseURL := fmt.Sprintf("https://objectstorage.%s.oraclecloud.com", region)

	// Combine with the PAR access URI
	fullURL := fmt.Sprintf("%s%s", baseURL, *response.AccessUri)

	p.logger.WithField("url", fullURL).
		WithField("expiry", expiry).
		Debug("Generated presigned URL")

	return fullURL, nil
}

// GetPresignedUploadURL generates a presigned URL for uploading an object
func (p *OCIProvider) GetPresignedUploadURL(ctx context.Context, uri string, expiry time.Duration) (string, error) {
	// Parse the URI
	ociURI, err := parseOCIURI(uri, p.namespace, p.bucket)
	if err != nil {
		return "", fmt.Errorf("failed to parse URI: %w", err)
	}

	// Create a pre-authenticated request for upload
	createPARRequest := objectstorage.CreatePreauthenticatedRequestRequest{
		NamespaceName: &ociURI.Namespace,
		BucketName:    &ociURI.Bucket,
		CreatePreauthenticatedRequestDetails: objectstorage.CreatePreauthenticatedRequestDetails{
			Name:       common.String(fmt.Sprintf("upload-%s-%d", ociURI.Object, time.Now().Unix())),
			ObjectName: &ociURI.Object,
			AccessType: objectstorage.CreatePreauthenticatedRequestDetailsAccessTypeObjectwrite,
			TimeExpires: &common.SDKTime{
				Time: time.Now().Add(expiry),
			},
		},
	}

	// Create the PAR
	response, err := p.client.CreatePreauthenticatedRequest(ctx, createPARRequest)
	if err != nil {
		return "", fmt.Errorf("failed to create pre-authenticated request: %w", err)
	}

	// Build the full URL
	region := p.region
	if region == "" {
		// Default to us-phoenix-1 if no region configured
		region = "us-phoenix-1"
		p.logger.Warn("No region configured, using default us-phoenix-1")
	}

	baseURL := fmt.Sprintf("https://objectstorage.%s.oraclecloud.com", region)
	fullURL := fmt.Sprintf("%s%s", baseURL, *response.AccessUri)

	p.logger.WithField("url", fullURL).
		WithField("expiry", expiry).
		Debug("Generated presigned upload URL")

	return fullURL, nil
}

// ListPresignedRequests lists all active pre-authenticated requests for a bucket
func (p *OCIProvider) ListPresignedRequests(ctx context.Context, bucketName string) ([]objectstorage.PreauthenticatedRequestSummary, error) {
	if bucketName == "" {
		bucketName = p.bucket
	}

	request := objectstorage.ListPreauthenticatedRequestsRequest{
		NamespaceName: &p.namespace,
		BucketName:    &bucketName,
	}

	response, err := p.client.ListPreauthenticatedRequests(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to list pre-authenticated requests: %w", err)
	}

	return response.Items, nil
}

// DeletePresignedRequest deletes a pre-authenticated request
func (p *OCIProvider) DeletePresignedRequest(ctx context.Context, bucketName string, parID string) error {
	if bucketName == "" {
		bucketName = p.bucket
	}

	request := objectstorage.DeletePreauthenticatedRequestRequest{
		NamespaceName: &p.namespace,
		BucketName:    &bucketName,
		ParId:         &parID,
	}

	_, err := p.client.DeletePreauthenticatedRequest(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to delete pre-authenticated request: %w", err)
	}

	return nil
}

// GeneratePresignedURLWithMethod generates a presigned URL for a specific HTTP method
// This is a lower-level method that provides more control
func (p *OCIProvider) GeneratePresignedURLWithMethod(ctx context.Context, uri string, method string, expiry time.Duration, headers map[string]string) (string, error) {
	// Parse the URI
	ociURI, err := parseOCIURI(uri, p.namespace, p.bucket)
	if err != nil {
		return "", fmt.Errorf("failed to parse URI: %w", err)
	}

	// Determine access type based on method
	var accessType objectstorage.CreatePreauthenticatedRequestDetailsAccessTypeEnum
	switch method {
	case http.MethodGet, http.MethodHead:
		accessType = objectstorage.CreatePreauthenticatedRequestDetailsAccessTypeObjectread
	case http.MethodPut, http.MethodPost:
		accessType = objectstorage.CreatePreauthenticatedRequestDetailsAccessTypeObjectwrite
	case http.MethodDelete:
		accessType = objectstorage.CreatePreauthenticatedRequestDetailsAccessTypeObjectreadwrite
	default:
		return "", fmt.Errorf("unsupported HTTP method: %s", method)
	}

	// Create the PAR
	createPARRequest := objectstorage.CreatePreauthenticatedRequestRequest{
		NamespaceName: &ociURI.Namespace,
		BucketName:    &ociURI.Bucket,
		CreatePreauthenticatedRequestDetails: objectstorage.CreatePreauthenticatedRequestDetails{
			Name:       common.String(fmt.Sprintf("%s-%s-%d", method, ociURI.Object, time.Now().Unix())),
			ObjectName: &ociURI.Object,
			AccessType: accessType,
			TimeExpires: &common.SDKTime{
				Time: time.Now().Add(expiry),
			},
		},
	}

	response, err := p.client.CreatePreauthenticatedRequest(ctx, createPARRequest)
	if err != nil {
		return "", fmt.Errorf("failed to create pre-authenticated request: %w", err)
	}

	// Build the full URL
	region := p.region
	if region == "" {
		// Default to us-phoenix-1 if no region configured
		region = "us-phoenix-1"
		p.logger.Warn("No region configured, using default us-phoenix-1")
	}

	baseURL := fmt.Sprintf("https://objectstorage.%s.oraclecloud.com", region)
	fullURL := fmt.Sprintf("%s%s", baseURL, *response.AccessUri)

	// Add query parameters for headers if needed
	if len(headers) > 0 {
		u, err := url.Parse(fullURL)
		if err != nil {
			return "", fmt.Errorf("failed to parse URL: %w", err)
		}

		q := u.Query()
		for k, v := range headers {
			q.Add(k, v)
		}
		u.RawQuery = q.Encode()
		fullURL = u.String()
	}

	return fullURL, nil
}
