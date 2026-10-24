package fakeclient

import (
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Calls counts the calls of one verb and keeps each call's key and resolved
// options. Build one with CountingVerb and pass it to New; it sees every call
// of its verb, including the ones a FailingVerb refuses.
type Calls struct {
	verbs []Verb
	scope any
	mu    sync.Mutex
	calls []Call
}

// Call is one recorded call: the key of the object it named (empty for List
// and DeleteAllOf) and its options resolved into the client's options struct
// for the verb (client.DeleteOptions for Delete, client.ListOptions for List,
// client.SubResourceUpdateOptions for StatusUpdate, and so on). OptionsOf
// reads them typed.
type Call struct {
	Key     client.ObjectKey
	Options any
}

// CountingVerb counts the calls of verb and records their keys and options.
func CountingVerb(verb Verb) *Calls {
	return &Calls{verbs: []Verb{verb}}
}

// CountingWrites counts the calls of every verb that changes the store:
// Create, Update, Patch, Apply, Delete, DeleteAllOf and the Status writes.
// A test of code that only reads asserts a count of zero.
func CountingWrites() *Calls {
	return &Calls{verbs: writeVerbs}
}

// OnKind narrows the counter to calls whose object has obj's type; for List
// hand the list type, for Apply the apply configuration type.
func (c *Calls) OnKind(obj any) *Calls {
	c.scope = obj
	return c
}

func (c *Calls) apply(cfg *config) {
	for _, verb := range c.verbs {
		cfg.observers[verb] = append(cfg.observers[verb], c)
	}
}

func (c *Calls) record(k call) {
	if !sameKind(c.scope, k.subject) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, Call{Key: k.key, Options: k.options})
}

// Count is the number of calls seen so far.
func (c *Calls) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.calls)
}

// Keys lists the key of every call in order.
func (c *Calls) Keys() []client.ObjectKey {
	c.mu.Lock()
	defer c.mu.Unlock()
	keys := make([]client.ObjectKey, 0, len(c.calls))
	for _, k := range c.calls {
		keys = append(keys, k.Key)
	}
	return keys
}

// All lists every call in order.
func (c *Calls) All() []Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Call(nil), c.calls...)
}

// OptionsOf lists the resolved options of every recorded call whose options
// have type T, in order: OptionsOf[client.DeleteOptions](deletes).
func OptionsOf[T any](c *Calls) []T {
	var out []T
	for _, k := range c.All() {
		if o, ok := k.Options.(T); ok {
			out = append(out, o)
		}
	}
	return out
}
