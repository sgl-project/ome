package effective

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/runtimerevision"
)

func inspectRuntimeRevision(
	revision *appsv1.ControllerRevision,
	expectedNamespace, expectedName, runtimeName string,
	sourceKind runtimerevision.SourceKind,
	sourceNamespace string,
) RuntimeRevisionObservation {
	observation := RuntimeRevisionObservation{
		Available: false, Consistency: RevisionConsistencyUnknown,
		expectedName: expectedName, expectedNamespace: expectedNamespace,
	}
	if revision == nil {
		return observation
	}

	observation.Name = revision.Name
	observation.Namespace = revision.Namespace
	observation.UID = string(revision.UID)
	observation.ResourceVersion = revision.ResourceVersion
	if revision.CreationTimestamp.Time.IsZero() {
		observation.CreationTimestamp = metav1.Time{}
	} else {
		observation.CreationTimestamp = metav1.NewTime(revision.CreationTimestamp.Time.UTC())
	}
	observation.Ordinal = revision.Revision
	observation.objectReturned = true
	observation.objectFingerprint = runtimeRevisionWriterFingerprint(revision)
	observation.SourceName = revision.Labels[constants.RuntimeRevisionOfLabelKey]
	observation.SourceKind = revision.Labels[constants.RuntimeRevisionOfKindLabelKey]
	observation.SourceNamespace = revision.Labels[constants.RuntimeRevisionOfNamespaceLabelKey]
	observation.rawShortHash = revision.Labels[constants.RuntimeRevisionHashLabelKey]

	addCode := func(code RevisionConsistencyCode) {
		for _, existing := range observation.consistencyCodes {
			if existing == code {
				return
			}
		}
		observation.consistencyCodes = append(observation.consistencyCodes, code)
	}
	if (expectedName != "" && revision.Name != expectedName) ||
		(expectedNamespace != "" && revision.Namespace != expectedNamespace) {
		addCode(RevisionConsistencyReturnedIdentity)
	}
	if revision.Annotations[constants.RuntimeRevisionCreatedByKey] != constants.RuntimeRevisionCreatedByOMEValue {
		addCode(RevisionConsistencyCreatedBy)
	}
	if observation.SourceName == "" || (runtimeName != "" && observation.SourceName != runtimeName) {
		addCode(RevisionConsistencySourceName)
	}
	observedKindValid := observation.SourceKind == string(runtimerevision.KindClusterServingRuntime) ||
		observation.SourceKind == string(runtimerevision.KindServingRuntime)
	if !observedKindValid || (sourceKind != "" && observation.SourceKind != string(sourceKind)) {
		addCode(RevisionConsistencySourceKind)
	}
	observedNamespaceValid := (observation.SourceKind == string(runtimerevision.KindClusterServingRuntime) && observation.SourceNamespace == "") ||
		(observation.SourceKind == string(runtimerevision.KindServingRuntime) && observation.SourceNamespace != "")
	if !observedNamespaceValid || (sourceKind != "" && observation.SourceNamespace != sourceNamespace) {
		addCode(RevisionConsistencySourceNamespace)
	}
	if revision.Revision != 1 {
		addCode(RevisionConsistencyOrdinal)
	}
	if revision.Data.Object != nil {
		addCode(RevisionConsistencyUnexpectedDataObject)
	}

	spec := &v1beta1.ServingRuntimeSpec{}
	if err := json.Unmarshal(revision.Data.Raw, spec); err != nil {
		addCode(RevisionConsistencyMalformedPayload)
		observation.Consistency = RevisionConsistencyInconsistent
		sortConsistencyCodes(observation.consistencyCodes)
		return observation
	}
	observation.usable = true
	observation.Available = true
	observation.spec = spec
	observation.Disabled = spec.IsDisabled()
	canonical, err := json.Marshal(spec)
	if err != nil || !bytes.Equal(canonical, revision.Data.Raw) || !strictRuntimeSpecJSON(revision.Data.Raw) {
		addCode(RevisionConsistencyPayloadCanonicality)
	}
	fullHash, shortHash, err := runtimerevision.Hash(spec)
	if err == nil {
		observation.fullHash = fullHash
		observation.computedShortHash = shortHash
		if !validShortHash(observation.rawShortHash) {
			addCode(RevisionConsistencyHashLabelInvalid)
		} else if observation.rawShortHash != shortHash {
			addCode(RevisionConsistencyHashLabelMismatch)
		} else {
			observation.ShortHash = shortHash
		}
		if observation.SourceName != "" && observedKindValid && observedNamespaceValid {
			expectedRevisionName := runtimerevision.Name(
				runtimerevision.SourceKind(observation.SourceKind), observation.SourceNamespace,
				observation.SourceName, shortHash,
			)
			if revision.Name != expectedRevisionName {
				addCode(RevisionConsistencyNameHash)
			}
		}
	} else {
		addCode(RevisionConsistencyHashLabelInvalid)
	}
	if len(observation.consistencyCodes) == 0 {
		observation.Consistency = RevisionConsistencyConsistent
	} else {
		observation.Consistency = RevisionConsistencyInconsistent
		sortConsistencyCodes(observation.consistencyCodes)
	}
	return observation
}

func runtimeRevisionWriterFingerprint(revision *appsv1.ControllerRevision) string {
	if revision == nil {
		return ""
	}
	digest := sha256.New()
	var length [8]byte
	writePart := func(value []byte) {
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		_, _ = digest.Write(length[:])
		_, _ = digest.Write(value)
	}
	writePresence := func(present bool) {
		if present {
			writePart([]byte{1})
			return
		}
		writePart([]byte{0})
	}

	writePart([]byte(revision.UID))
	createdAt := revision.CreationTimestamp.Time
	writePresence(!createdAt.IsZero())
	if !createdAt.IsZero() {
		var seconds [8]byte
		binary.BigEndian.PutUint64(seconds[:], uint64(createdAt.Unix()))
		writePart(seconds[:])
		var nanoseconds [4]byte
		binary.BigEndian.PutUint32(nanoseconds[:], uint32(createdAt.Nanosecond()))
		writePart(nanoseconds[:])
	}
	writePresence(revision.Labels != nil)
	labelKeys := make([]string, 0, len(revision.Labels))
	for key := range revision.Labels {
		labelKeys = append(labelKeys, key)
	}
	sort.Strings(labelKeys)
	var labelCount [8]byte
	binary.BigEndian.PutUint64(labelCount[:], uint64(len(labelKeys)))
	writePart(labelCount[:])
	for _, key := range labelKeys {
		writePart([]byte(key))
		writePart([]byte(revision.Labels[key]))
	}
	writePresence(revision.Data.Raw != nil)
	writePart(revision.Data.Raw)
	if revision.Data.Object == nil {
		writePresence(false)
	} else {
		encoded, err := json.Marshal(revision.Data.Object)
		if err != nil {
			return ""
		}
		writePresence(true)
		writePart([]byte(fmt.Sprintf("%T", revision.Data.Object)))
		writePart(encoded)
	}
	var ordinal [8]byte
	binary.BigEndian.PutUint64(ordinal[:], uint64(revision.Revision))
	writePart(ordinal[:])
	createdBy, present := revision.Annotations[constants.RuntimeRevisionCreatedByKey]
	writePresence(present)
	writePart([]byte(createdBy))
	return fmt.Sprintf("%x", digest.Sum(nil))
}

func strictRuntimeSpecJSON(raw []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var spec v1beta1.ServingRuntimeSpec
	if err := decoder.Decode(&spec); err != nil {
		return false
	}
	return errors.Is(decoder.Decode(&struct{}{}), io.EOF)
}

func validShortHash(hash string) bool {
	if len(hash) != 8 {
		return false
	}
	for _, character := range hash {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func sortConsistencyCodes(codes []RevisionConsistencyCode) {
	order := map[RevisionConsistencyCode]int{
		RevisionConsistencyReturnedIdentity:     0,
		RevisionConsistencyCreatedBy:            1,
		RevisionConsistencySourceName:           2,
		RevisionConsistencySourceKind:           3,
		RevisionConsistencySourceNamespace:      4,
		RevisionConsistencyHashLabelInvalid:     5,
		RevisionConsistencyHashLabelMismatch:    6,
		RevisionConsistencyNameHash:             7,
		RevisionConsistencyOrdinal:              8,
		RevisionConsistencyUnexpectedDataObject: 9,
		RevisionConsistencyPayloadCanonicality:  10,
		RevisionConsistencyMalformedPayload:     11,
		RevisionConsistencyDuplicateIdentity:    12,
		RevisionConsistencyConflictingIdentity:  13,
		RevisionConsistencyDuplicateContentHash: 14,
		RevisionConsistencyShortHashCollision:   15,
	}
	sort.SliceStable(codes, func(i, j int) bool { return order[codes[i]] < order[codes[j]] })
}

// RequireActive returns a defensive copy of the controller-selected active
// configuration or a fixed error when controller evidence does not select one.
func (s *RuntimeState) RequireActive() (*ActiveConfiguration, error) {
	if s == nil || s.active == nil {
		return nil, ErrActiveRuntimeUnavailable
	}
	return cloneActiveConfiguration(s.active), nil
}

// RequireConsistentActive is the mutation-safety gate. Controller-readable
// revision data can remain active while failing this stronger writer contract.
func (s *RuntimeState) RequireConsistentActive() (*ActiveConfiguration, error) {
	active, err := s.RequireActive()
	if err != nil {
		return nil, err
	}
	if active.Consistency != RevisionConsistencyConsistent {
		return nil, ErrActiveRuntimeInconsistent
	}
	return active, nil
}

func cloneActiveConfiguration(active *ActiveConfiguration) *ActiveConfiguration {
	if active == nil {
		return nil
	}
	cloned := *active
	cloned.components = cloneEffectiveComponents(active.components)
	if active.spec != nil {
		cloned.spec = active.spec.DeepCopy()
	}
	return &cloned
}

// Components returns defensive copies of the merged component summaries and
// their private typed specs.
func (a *ActiveConfiguration) Components() []EffectiveComponent {
	if a == nil {
		return []EffectiveComponent{}
	}
	return cloneEffectiveComponents(a.components)
}

// Roles returns the deterministic set of evidence roles for this revision.
func (o RuntimeRevisionObservation) Roles() []RuntimeRevisionRole {
	return append([]RuntimeRevisionRole{}, o.roles...)
}

// ConsistencyCodes returns a defensive copy of bounded writer-contract codes.
func (o RuntimeRevisionObservation) ConsistencyCodes() []RevisionConsistencyCode {
	return append([]RevisionConsistencyCode{}, o.consistencyCodes...)
}

// ObjectReturned reports whether the API supplied a ControllerRevision object,
// including when its payload or returned identity was malformed.
func (o RuntimeRevisionObservation) ObjectReturned() bool {
	return o.objectReturned
}

// ExpectedName returns the exact ControllerRevision GET key associated with
// this evidence. It is empty for history-only observations.
func (o RuntimeRevisionObservation) ExpectedName() string {
	return o.expectedName
}

// ExpectedNamespace returns the namespace used for the exact GET or bounded
// history LIST that produced this evidence.
func (o RuntimeRevisionObservation) ExpectedNamespace() string {
	return o.expectedNamespace
}

// ReturnedName returns the metadata.name supplied by the API response. It is
// empty when an exact GET returned no object.
func (o RuntimeRevisionObservation) ReturnedName() string {
	if !o.objectReturned {
		return ""
	}
	return o.Name
}

// ReturnedNamespace returns metadata.namespace from the API response. It is
// empty when an exact GET returned no object.
func (o RuntimeRevisionObservation) ReturnedNamespace() string {
	if !o.objectReturned {
		return ""
	}
	return o.Namespace
}

// LiveConfiguration returns a defensive copy of the independently resolved
// live snapshot, or nil when live evidence was unavailable.
func (s *RuntimeState) LiveConfiguration() *LiveConfiguration {
	if s == nil {
		return nil
	}
	return cloneLiveConfiguration(s.live)
}

// RevisionObservations returns defensive copies of safe revision provenance.
func (s *RuntimeState) RevisionObservations() []RuntimeRevisionObservation {
	if s == nil {
		return []RuntimeRevisionObservation{}
	}
	observations := make([]RuntimeRevisionObservation, len(s.revisions))
	for i := range s.revisions {
		observations[i] = s.revisions[i]
		observations[i].roles = s.revisions[i].Roles()
		observations[i].consistencyCodes = s.revisions[i].ConsistencyCodes()
		if s.revisions[i].spec != nil {
			observations[i].spec = s.revisions[i].spec.DeepCopy()
		}
	}
	return observations
}

// SourceIssues returns fixed-code evidence errors with their causes preserved
// for errors.Is/errors.As diagnostics.
func (s *RuntimeState) SourceIssues() []RuntimeSourceIssue {
	if s == nil {
		return []RuntimeSourceIssue{}
	}
	return append([]RuntimeSourceIssue{}, s.issues...)
}

// LiveAvailability returns the bounded outcome of independent live runtime
// resolution without exposing the source error or runtime specification.
func (s *RuntimeState) LiveAvailability() LiveRuntimeAvailability {
	if s == nil {
		return LiveRuntimeUnavailable
	}
	switch s.liveAvailability {
	case liveAvailable:
		return LiveRuntimeAvailable
	case liveNotFound:
		return LiveRuntimeNotFound
	case liveDisabled:
		return LiveRuntimeDisabled
	default:
		return LiveRuntimeUnavailable
	}
}

// InferenceServiceIdentity returns the safe primary object identity whose
// snapshot produced this runtime evidence. Call MatchesInferenceService when
// exact snapshot validation is required.
func (s *RuntimeState) InferenceServiceIdentity() InferenceServiceIdentity {
	if s == nil {
		return InferenceServiceIdentity{}
	}
	return s.inferenceService.identity
}

// MatchesInferenceService reports whether candidate is the exact Kubernetes
// object snapshot used for collection. ResourceVersion participates in the
// comparison but is deliberately never exposed by the state API.
func (s *RuntimeState) MatchesInferenceService(candidate *v1beta1.InferenceService) bool {
	if s == nil || candidate == nil {
		return false
	}
	identity := s.inferenceService.identity
	if identity.Name == "" || identity.Namespace == "" || identity.UID == "" ||
		s.inferenceService.resourceVersion == "" {
		return false
	}
	return identity.Name == candidate.Name &&
		identity.Namespace == candidate.Namespace &&
		identity.UID == string(candidate.UID) &&
		s.inferenceService.resourceVersion == candidate.ResourceVersion
}

// HistoryNamespace returns the namespace searched for ControllerRevision
// history. It is empty when no history LIST was attempted.
func (s *RuntimeState) HistoryNamespace() string {
	if s == nil {
		return ""
	}
	return s.historyNamespace
}

func cloneLiveConfiguration(live *LiveConfiguration) *LiveConfiguration {
	if live == nil {
		return nil
	}
	cloned := *live
	if live.Model != nil {
		model := *live.Model
		if live.Model.spec != nil {
			model.spec = live.Model.spec.DeepCopy()
		}
		cloned.Model = &model
	}
	if live.Runtime.spec != nil {
		cloned.Runtime.spec = live.Runtime.spec.DeepCopy()
	}
	cloned.Runtime.DeclaredInheritance.chain = append(
		[]RuntimeSourceReference{}, live.Runtime.DeclaredInheritance.chain...,
	)
	cloned.Components = cloneEffectiveComponents(live.Components)
	cloned.Advisories = append([]RuntimeAdvisory{}, live.Advisories...)
	return &cloned
}

func cloneEffectiveComponents(components []EffectiveComponent) []EffectiveComponent {
	cloned := make([]EffectiveComponent, len(components))
	for i := range components {
		cloned[i] = components[i]
		if components[i].engine != nil {
			cloned[i].engine = components[i].engine.DeepCopy()
		}
		if components[i].decoder != nil {
			cloned[i].decoder = components[i].decoder.DeepCopy()
		}
		if components[i].router != nil {
			cloned[i].router = components[i].router.DeepCopy()
		}
	}
	return cloned
}
