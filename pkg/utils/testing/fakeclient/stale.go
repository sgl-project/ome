package fakeclient

import (
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StaleRead answers Get and List from the copies handed in, whatever the
// store holds: a Get of a copy's key returns the copy, even after the store
// deleted the object, and a List replaces the items of those keys with the
// copies and adds none. That is how a lagging cache reads.
func StaleRead(objs ...client.Object) Option {
	return optionFunc(func(c *config) { c.stale = append(c.stale, objs...) })
}

// staleFor returns the stale copy of obj's type under key, or nil.
func (c *config) staleFor(obj client.Object, key client.ObjectKey) client.Object {
	for _, s := range c.stale {
		if reflect.TypeOf(s) == reflect.TypeOf(obj) && client.ObjectKeyFromObject(s) == key {
			return s
		}
	}
	return nil
}

// copyInto overwrites dst with a deep copy of src; both have the same type.
func copyInto(dst, src client.Object) {
	reflect.ValueOf(dst).Elem().Set(reflect.ValueOf(src.DeepCopyObject()).Elem())
}

// replaceStaleItems swaps every listed item that has a stale copy for that
// copy and leaves the rest of the list as the store answered it.
func (c *config) replaceStaleItems(list client.ObjectList) error {
	if len(c.stale) == 0 {
		return nil
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		return err
	}
	changed := false
	for i, item := range items {
		obj, ok := item.(client.Object)
		if !ok {
			continue
		}
		if stale := c.staleFor(obj, client.ObjectKeyFromObject(obj)); stale != nil {
			items[i] = stale.DeepCopyObject()
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return meta.SetList(list, items)
}
