package gcp

import (
	"context"
	"io"

	"cloud.google.com/go/storage"
	"github.com/sgl-project/ome/pkg/auth"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
)

// gcpCredentials defines the interface for GCP-specific credentials methods
type gcpCredentials interface {
	auth.Credentials
	GetTokenSource() oauth2.TokenSource
}

// gcsClient defines the interface for GCS client operations
type gcsClient interface {
	Bucket(name string) gcsBucketHandle
	Close() error
}

// gcsBucketHandle defines the interface for GCS bucket operations
type gcsBucketHandle interface {
	Object(name string) gcsObjectHandle
	Objects(ctx context.Context, q *storage.Query) gcsObjectIterator
}

// gcsObjectHandle defines the interface for GCS object operations
type gcsObjectHandle interface {
	NewWriter(ctx context.Context) gcsWriter
	NewReader(ctx context.Context) (gcsReader, error)
	NewRangeReader(ctx context.Context, offset, length int64) (gcsReader, error)
	Delete(ctx context.Context) error
	Attrs(ctx context.Context) (*storage.ObjectAttrs, error)
	CopierFrom(src gcsObjectHandle) gcsCopier
	ComposerFrom(srcs ...*storage.ObjectHandle) gcsComposer
}

// gcsWriter defines the interface for GCS object writer
type gcsWriter interface {
	io.WriteCloser
}

// gcsReader defines the interface for GCS object reader
type gcsReader interface {
	io.ReadCloser
}

// gcsCopier defines the interface for GCS object copier
type gcsCopier interface {
	Run(ctx context.Context) (*storage.ObjectAttrs, error)
}

// gcsComposer defines the interface for GCS object composer
type gcsComposer interface {
	Run(ctx context.Context) (*storage.ObjectAttrs, error)
}

// gcsObjectIterator defines the interface for iterating over GCS objects
type gcsObjectIterator interface {
	Next() (*storage.ObjectAttrs, error)
}

// Wrapper types to implement the interfaces

type clientWrapper struct {
	*storage.Client
}

func (c *clientWrapper) Bucket(name string) gcsBucketHandle {
	return &bucketWrapper{c.Client.Bucket(name)}
}

func (c *clientWrapper) Close() error {
	return c.Client.Close()
}

type bucketWrapper struct {
	*storage.BucketHandle
}

func (b *bucketWrapper) Object(name string) gcsObjectHandle {
	return &objectWrapper{b.BucketHandle.Object(name)}
}

func (b *bucketWrapper) Objects(ctx context.Context, q *storage.Query) gcsObjectIterator {
	return b.BucketHandle.Objects(ctx, q)
}

type objectWrapper struct {
	*storage.ObjectHandle
}

func (o *objectWrapper) NewWriter(ctx context.Context) gcsWriter {
	return &writerWrapper{o.ObjectHandle.NewWriter(ctx)}
}

func (o *objectWrapper) NewReader(ctx context.Context) (gcsReader, error) {
	return o.ObjectHandle.NewReader(ctx)
}

func (o *objectWrapper) NewRangeReader(ctx context.Context, offset, length int64) (gcsReader, error) {
	return o.ObjectHandle.NewRangeReader(ctx, offset, length)
}

func (o *objectWrapper) Delete(ctx context.Context) error {
	return o.ObjectHandle.Delete(ctx)
}

func (o *objectWrapper) Attrs(ctx context.Context) (*storage.ObjectAttrs, error) {
	return o.ObjectHandle.Attrs(ctx)
}

func (o *objectWrapper) CopierFrom(src gcsObjectHandle) gcsCopier {
	if srcWrapper, ok := src.(*objectWrapper); ok {
		return o.ObjectHandle.CopierFrom(srcWrapper.ObjectHandle)
	}
	return nil
}

func (o *objectWrapper) ComposerFrom(srcs ...*storage.ObjectHandle) gcsComposer {
	return o.ObjectHandle.ComposerFrom(srcs...)
}

type writerWrapper struct {
	*storage.Writer
}

func (w *writerWrapper) Write(p []byte) (n int, err error) {
	return w.Writer.Write(p)
}

func (w *writerWrapper) Close() error {
	return w.Writer.Close()
}

// Helper to check if an error is a not found error
func isNotFoundErr(err error) bool {
	return err == storage.ErrObjectNotExist ||
		err == storage.ErrBucketNotExist ||
		err == iterator.Done
}
