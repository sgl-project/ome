package fakeclient

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// Verb names one client call a behaviour acts on.
type Verb string

// The verbs a behaviour can name. The Status verbs are the calls made
// through Status(); other subresources pass through untouched.
const (
	Get          Verb = "Get"
	List         Verb = "List"
	Create       Verb = "Create"
	Update       Verb = "Update"
	Patch        Verb = "Patch"
	Apply        Verb = "Apply"
	Delete       Verb = "Delete"
	DeleteAllOf  Verb = "DeleteAllOf"
	StatusUpdate Verb = "StatusUpdate"
	StatusPatch  Verb = "StatusPatch"
	StatusApply  Verb = "StatusApply"
)

var knownVerbs = map[Verb]bool{
	Get: true, List: true, Create: true, Update: true, Patch: true, Apply: true,
	Delete: true, DeleteAllOf: true, StatusUpdate: true, StatusPatch: true, StatusApply: true,
}

// writeVerbs are the verbs that change the store; CountingWrites and
// FailingWrites act on all of them.
var writeVerbs = []Verb{Create, Update, Patch, Apply, Delete, DeleteAllOf, StatusUpdate, StatusPatch, StatusApply}

// statusSubresource is the subresource name the Status() writer uses.
const statusSubresource = "status"

// call is one client call as the behaviours see it: the verb, the object,
// list or apply configuration it names, the key of that object, and the
// options resolved into the client's options struct for the verb.
type call struct {
	verb    Verb
	subject any
	key     client.ObjectKey
	options any
}

// before shows a call to every counter and every refusing rule of its verb,
// so each keeps its own count of all calls, and returns the first refusal.
func (c *config) before(k call) error {
	for _, o := range c.observers[k.verb] {
		o.record(k)
	}
	var refusal error
	for _, f := range c.failures[k.verb] {
		if err := f.refuse(k); err != nil && refusal == nil {
			refusal = err
		}
	}
	return refusal
}

// sameKind reports whether a behaviour scoped to scope's type applies to the
// subject of a call; an unscoped behaviour applies to every subject.
func sameKind(scope, subject any) bool {
	return scope == nil || reflect.TypeOf(scope) == reflect.TypeOf(subject)
}

// applyKey reads the name and namespace of a typed apply configuration; a
// configuration without them has an empty key.
func applyKey(obj runtime.ApplyConfiguration) client.ObjectKey {
	named, ok := obj.(interface {
		GetName() *string
		GetNamespace() *string
	})
	if !ok {
		return client.ObjectKey{}
	}
	var key client.ObjectKey
	if name := named.GetName(); name != nil {
		key.Name = *name
	}
	if ns := named.GetNamespace(); ns != nil {
		key.Namespace = *ns
	}
	return key
}

// hooks is the one interceptor layer that runs the behaviours: count, refuse,
// answer from the store, then replace a read with its stale copy.
func (c *config) hooks() interceptor.Funcs {
	return interceptor.Funcs{
		Get: func(ctx context.Context, next client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			o := client.GetOptions{}
			o.ApplyOptions(opts)
			if err := c.before(call{Get, obj, key, o}); err != nil {
				return err
			}
			err := next.Get(ctx, key, obj, opts...)
			if stale := c.staleFor(obj, key); stale != nil {
				copyInto(obj, stale)
				return nil
			}
			return err
		},
		List: func(ctx context.Context, next client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			o := client.ListOptions{}
			o.ApplyOptions(opts)
			if err := c.before(call{List, list, client.ObjectKey{}, o}); err != nil {
				return err
			}
			if err := next.List(ctx, list, opts...); err != nil {
				return err
			}
			return c.replaceStaleItems(list)
		},
		Create: func(ctx context.Context, next client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			o := client.CreateOptions{}
			o.ApplyOptions(opts)
			if err := c.before(call{Create, obj, client.ObjectKeyFromObject(obj), o}); err != nil {
				return err
			}
			return next.Create(ctx, obj, opts...)
		},
		Update: func(ctx context.Context, next client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			o := client.UpdateOptions{}
			o.ApplyOptions(opts)
			if err := c.before(call{Update, obj, client.ObjectKeyFromObject(obj), o}); err != nil {
				return err
			}
			return next.Update(ctx, obj, opts...)
		},
		Patch: func(ctx context.Context, next client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			o := client.PatchOptions{}
			o.ApplyOptions(opts)
			if err := c.before(call{Patch, obj, client.ObjectKeyFromObject(obj), o}); err != nil {
				return err
			}
			return next.Patch(ctx, obj, patch, opts...)
		},
		Apply: func(ctx context.Context, next client.WithWatch, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
			o := client.ApplyOptions{}
			o.ApplyOptions(opts)
			if err := c.before(call{Apply, obj, applyKey(obj), o}); err != nil {
				return err
			}
			return next.Apply(ctx, obj, opts...)
		},
		Delete: func(ctx context.Context, next client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			o := client.DeleteOptions{}
			o.ApplyOptions(opts)
			if err := c.before(call{Delete, obj, client.ObjectKeyFromObject(obj), o}); err != nil {
				return err
			}
			return next.Delete(ctx, obj, opts...)
		},
		DeleteAllOf: func(ctx context.Context, next client.WithWatch, obj client.Object, opts ...client.DeleteAllOfOption) error {
			o := client.DeleteAllOfOptions{}
			o.ApplyOptions(opts)
			if err := c.before(call{DeleteAllOf, obj, client.ObjectKey{}, o}); err != nil {
				return err
			}
			return next.DeleteAllOf(ctx, obj, opts...)
		},
		SubResourceUpdate: func(ctx context.Context, next client.Client, name string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			if name == statusSubresource {
				o := client.SubResourceUpdateOptions{}
				o.ApplyOptions(opts)
				if err := c.before(call{StatusUpdate, obj, client.ObjectKeyFromObject(obj), o}); err != nil {
					return err
				}
			}
			return next.SubResource(name).Update(ctx, obj, opts...)
		},
		SubResourcePatch: func(ctx context.Context, next client.Client, name string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			if name == statusSubresource {
				o := client.SubResourcePatchOptions{}
				o.ApplyOptions(opts)
				if err := c.before(call{StatusPatch, obj, client.ObjectKeyFromObject(obj), o}); err != nil {
					return err
				}
			}
			return next.SubResource(name).Patch(ctx, obj, patch, opts...)
		},
		SubResourceApply: func(ctx context.Context, next client.Client, name string, obj runtime.ApplyConfiguration, opts ...client.SubResourceApplyOption) error {
			if name == statusSubresource {
				o := client.SubResourceApplyOptions{}
				o.ApplyOpts(opts)
				if err := c.before(call{StatusApply, obj, applyKey(obj), o}); err != nil {
					return err
				}
			}
			return next.SubResource(name).Apply(ctx, obj, opts...)
		},
	}
}
