package fakeclient

import "sync"

// EveryCall makes FailingVerb refuse every call of its verb.
const EveryCall = 0

// Failure refuses the nth call of one verb (1-based, counted from the
// client's construction over the calls in its scope) or every call when nth
// is EveryCall. A refused call never reaches the store; counters still see it.
type Failure struct {
	verbs []Verb
	nth   int
	err   error
	scope any
	mu    sync.Mutex
	seen  int
}

// FailingVerb refuses the nth call of verb with err, or every call when nth
// is EveryCall.
func FailingVerb(verb Verb, nth int, err error) *Failure {
	return &Failure{verbs: []Verb{verb}, nth: nth, err: err}
}

// FailingWrites refuses the nth call of any verb that changes the store,
// counted over all of them, or every write when nth is EveryCall. Reads are
// answered as usual.
func FailingWrites(nth int, err error) *Failure {
	return &Failure{verbs: writeVerbs, nth: nth, err: err}
}

// OnKind narrows the rule to calls whose object has obj's type; for List hand
// the list type, for Apply the apply configuration type.
func (f *Failure) OnKind(obj any) *Failure {
	f.scope = obj
	return f
}

func (f *Failure) apply(cfg *config) {
	for _, verb := range f.verbs {
		cfg.failures[verb] = append(cfg.failures[verb], f)
	}
}

func (f *Failure) refuse(k call) error {
	if !sameKind(f.scope, k.subject) {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seen++
	if f.nth == EveryCall || f.seen == f.nth {
		return f.err
	}
	return nil
}
