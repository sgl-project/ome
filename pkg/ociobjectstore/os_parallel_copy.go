package ociobjectstore

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// CopyObjectAndWait performs an object copy using the OCI Object Storage SDK.
//
// This function initiates the copy and polls the work request until the operation is finished (success or failure),
// so it blocks and waits until the copy is complete before returning.
// Additionally, it copies all user-defined metadata from the source to the destination by default.
//
// If client.RetryPolicy() is nil, the SDK defaults to common.DefaultRetryPolicy(),
// which performs up to 8 attempts with exponential backoff (starting at 1s),
// retrying on HTTP 5xx, 429 (Too Many Requests), and transient network errors.
func (cds *OCIOSDataStore) CopyObjectAndWait(
	ctx context.Context,
	source, target ObjectURI,
	detailsOpts []func(*objectstorage.CopyObjectDetails),
	reqOpts []func(*objectstorage.CopyObjectRequest),
) error {
	details, err := buildDetails(source, target, detailsOpts...)
	if err != nil {
		return fmt.Errorf("buildDetails: %w", err)
	}
	req, err := buildRequest(source, details, reqOpts...)
	if err != nil {
		return fmt.Errorf("buildRequest: %w", err)
	}
	cds.logger.Infof("Starting CopyObject from source %v in region %s to target %v",
		source, cds.Config.Region, target)
	resp, err := cds.Client.CopyObject(ctx, req)
	if err != nil {
		return fmt.Errorf("CopyObject: %w", err)
	}
	workRequestID := *resp.OpcWorkRequestId
	cds.logger.Infof("Submitted Workrequest for CopyObject. Source: %v; Target: %v; Workrequest: %s", source, target, workRequestID)

	err = cds.waitForWorkRequest(ctx, workRequestID)
	if err != nil {
		return fmt.Errorf("copyObject failed. Source: %v; Target: %v; Workrequest: %s; Error: %w", source, target, workRequestID, err)
	}
	cds.logger.Infof("CopyObject from %v to %v completed successfully.", source, target)
	return nil
}

func (cds *OCIOSDataStore) waitForWorkRequest(ctx context.Context, workRequestID string) error {
	// Apply default timeout if none is set on context
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		cds.logger.Warnf("No context deadline provided; applying default timeout of %s", defaultTimeout)
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	}

	start := time.Now()

	// Configure exponential backoff with jitter
	backoffWithCtx := backoff.WithContext(
		backoff.NewExponentialBackOff(
			backoff.WithInitialInterval(retryDelay),
			backoff.WithMaxInterval(MaxInterval),
			backoff.WithMaxElapsedTime(MaxElapsedTime), // rely on context instead
		),
		ctx)

	retryErr := backoff.Retry(func() error {
		req := objectstorage.GetWorkRequestRequest{
			WorkRequestId: common.String(workRequestID),
		}

		resp, err := cds.Client.GetWorkRequest(ctx, req)
		if err != nil {
			cds.logger.Warnf("Retryable error polling work request %s: %v", workRequestID, err)
			return err // retry on transient error
		}

		switch resp.WorkRequest.Status {
		case objectstorage.WorkRequestStatusCompleted:
			cds.logger.Infof("Work request %s completed after %s", workRequestID, time.Since(start))
			return nil
		case objectstorage.WorkRequestStatusFailed:
			return backoff.Permanent(fmt.Errorf("work request %s failed: %v", workRequestID, resp.WorkRequest))
		case objectstorage.WorkRequestStatusCanceled:
			return backoff.Permanent(fmt.Errorf("work request %s was canceled", workRequestID))
		default:
			cds.logger.Debugf("Polling work request %s: current status %s", workRequestID, resp.WorkRequest.Status)
			return fmt.Errorf("work request %s still in progress", workRequestID)
		}
	}, backoffWithCtx)

	if retryErr != nil {
		return fmt.Errorf("wait for work request %s failed after %s: %w", workRequestID, time.Since(start), retryErr)
	}

	return nil
}

func buildDetails(
	source, target ObjectURI,
	opts ...func(*objectstorage.CopyObjectDetails),
) (objectstorage.CopyObjectDetails, error) {
	if source.ObjectName == "" || target.ObjectName == "" {
		return objectstorage.CopyObjectDetails{}, fmt.Errorf("source and target ObjectName required")
	}
	if target.Region == "" || target.BucketName == "" || target.Namespace == "" {
		return objectstorage.CopyObjectDetails{}, fmt.Errorf("target region, bucketName, namespace required")
	}
	details := objectstorage.CopyObjectDetails{
		SourceObjectName:      common.String(source.ObjectName),
		DestinationRegion:     common.String(target.Region),
		DestinationNamespace:  common.String(target.Namespace),
		DestinationBucket:     common.String(target.BucketName),
		DestinationObjectName: common.String(target.ObjectName),
	}
	for _, opt := range opts {
		opt(&details)
	}
	return details, nil
}

func buildRequest(
	source ObjectURI,
	details objectstorage.CopyObjectDetails,
	opts ...func(*objectstorage.CopyObjectRequest),
) (objectstorage.CopyObjectRequest, error) {
	if source.BucketName == "" || source.Namespace == "" {
		return objectstorage.CopyObjectRequest{}, fmt.Errorf("source BucketName and NamespaceName required")
	}
	req := objectstorage.CopyObjectRequest{
		NamespaceName:     common.String(source.Namespace),
		BucketName:        common.String(source.BucketName),
		CopyObjectDetails: details,
	}
	for _, opt := range opts {
		opt(&req)
	}
	return req, nil
}
