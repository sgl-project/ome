package replicator

import (
	"context"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/ome/pkg/xet"

	testingPkg "sigs.k8s.io/ome/pkg/testing"
)

func TestDownloadSnapshotWithTimeouts_EndToEndTimeout(t *testing.T) {
	origDownloadSnapHook := downloadSnapHook
	t.Cleanup(func() {
		downloadSnapHook = origDownloadSnapHook
	})

	downloadSnapHook = func(ctx context.Context, client *xet.Client, req *xet.SnapshotRequest) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}

	_, err := downloadSnapshotWithTimeouts(nil, &xet.SnapshotRequest{
		RepoID:   "org/model",
		Revision: "main",
	}, hfDownloadOptions{
		DownloadTimeout: 10 * time.Millisecond,
	}, testingPkg.SetupMockLogger())

	if err == nil || !strings.Contains(err.Error(), "exceeded timeout") {
		t.Fatalf("expected end-to-end timeout, got %v", err)
	}
}

func TestDownloadSnapshotWithTimeouts_StaleProgressTimeout(t *testing.T) {
	origDownloadSnapHook := downloadSnapHook
	origSetProgressHandlerHook := setProgressHandlerHook
	origDisableProgressHook := disableProgressHook
	t.Cleanup(func() {
		downloadSnapHook = origDownloadSnapHook
		setProgressHandlerHook = origSetProgressHandlerHook
		disableProgressHook = origDisableProgressHook
	})

	setProgressHandlerHook = func(client *xet.Client, handler xet.ProgressHandler, throttle time.Duration) error {
		return nil
	}
	disableProgressHook = func(client *xet.Client) error {
		return nil
	}
	downloadSnapHook = func(ctx context.Context, client *xet.Client, req *xet.SnapshotRequest) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}

	_, err := downloadSnapshotWithTimeouts(nil, &xet.SnapshotRequest{
		RepoID:   "org/model",
		Revision: "main",
	}, hfDownloadOptions{
		DownloadTimeout:      time.Second,
		StaleProgressTimeout: 10 * time.Millisecond,
	}, testingPkg.SetupMockLogger())

	if err == nil || !strings.Contains(err.Error(), "no bytes/files progress") {
		t.Fatalf("expected stale-progress timeout, got %v", err)
	}
}

func TestDownloadSnapshotWithTimeouts_ProgressPreventsStaleTimeout(t *testing.T) {
	origDownloadSnapHook := downloadSnapHook
	origSetProgressHandlerHook := setProgressHandlerHook
	origDisableProgressHook := disableProgressHook
	t.Cleanup(func() {
		downloadSnapHook = origDownloadSnapHook
		setProgressHandlerHook = origSetProgressHandlerHook
		disableProgressHook = origDisableProgressHook
	})

	var progressHandler xet.ProgressHandler
	setProgressHandlerHook = func(client *xet.Client, handler xet.ProgressHandler, throttle time.Duration) error {
		progressHandler = handler
		return nil
	}
	disableProgressHook = func(client *xet.Client) error {
		return nil
	}
	downloadSnapHook = func(ctx context.Context, client *xet.Client, req *xet.SnapshotRequest) (string, error) {
		for i := uint64(1); i <= 5; i++ {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(15 * time.Millisecond):
				progressHandler(xet.ProgressUpdate{CompletedBytes: i})
			}
		}
		return "/tmp/model", nil
	}

	path, err := downloadSnapshotWithTimeouts(nil, &xet.SnapshotRequest{
		RepoID:   "org/model",
		Revision: "main",
	}, hfDownloadOptions{
		DownloadTimeout:      time.Second,
		StaleProgressTimeout: 50 * time.Millisecond,
	}, testingPkg.SetupMockLogger())

	if err != nil {
		t.Fatalf("expected progress to prevent stale timeout, got %v", err)
	}
	if path != "/tmp/model" {
		t.Fatalf("unexpected path: got %s", path)
	}
}
