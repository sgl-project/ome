package fakeclient

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// EditedRead hands every Get or List answer whose type is T to edit before
// the caller sees it: T is the object type for Get and the list type for
// List. The store keeps what it holds, and a read a FailingVerb refuses is
// not edited. That is how a test shows the engine an input that changes
// between two reads, or a listing that is incomplete or out of order.
func EditedRead[T any](edit func(T)) Option {
	return WithInterceptors(interceptor.Funcs{
		Get: func(ctx context.Context, next client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if err := next.Get(ctx, key, obj, opts...); err != nil {
				return err
			}
			if subject, ok := obj.(T); ok {
				edit(subject)
			}
			return nil
		},
		List: func(ctx context.Context, next client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if err := next.List(ctx, list, opts...); err != nil {
				return err
			}
			if subject, ok := list.(T); ok {
				edit(subject)
			}
			return nil
		},
	})
}
