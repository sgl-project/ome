// Traffic-management validation.
//
// Webhook-level validation for the TrafficSpec typed core (LB
// algorithm, hash key, endpoint override). Annotation parsing and
// rollout validation live in sibling files.
package validation

import (
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// ValidateTrafficSpec runs the typed-core validation rules for
// InferenceService.spec.traffic. Returns nil when traffic is unset or
// internally consistent.
func ValidateTrafficSpec(t *v1beta1.TrafficSpec) error {
	if t == nil {
		return nil
	}

	if err := validateConsistentHashCoupling(t); err != nil {
		return err
	}
	if err := validateConsistentHashShape(t.ConsistentHash); err != nil {
		return err
	}
	if err := validateEndpointOverride(t.EndpointOverride); err != nil {
		return err
	}

	return nil
}

// validateConsistentHashCoupling enforces that ConsistentHash sub-block
// is present iff Algorithm == ConsistentHash.
func validateConsistentHashCoupling(t *v1beta1.TrafficSpec) error {
	isConsistentHash := t.Algorithm != nil && *t.Algorithm == v1beta1.LoadBalancingTypeConsistentHash
	switch {
	case isConsistentHash && t.ConsistentHash == nil:
		return fmt.Errorf("spec.traffic.consistentHash is required when spec.traffic.algorithm=ConsistentHash (MissingConsistentHashSpec)")
	case !isConsistentHash && t.ConsistentHash != nil:
		return fmt.Errorf("spec.traffic.consistentHash is only valid when spec.traffic.algorithm=ConsistentHash (UnexpectedConsistentHashSpec)")
	}
	return nil
}

// validateConsistentHashShape enforces the exactly-one-source rule for
// the ConsistentHash sub-block. Headers (one or more) when
// Type=Header, Cookie when Type=Cookie, nothing else when
// Type=SourceIP.
func validateConsistentHashShape(c *v1beta1.ConsistentHashSpec) error {
	if c == nil {
		return nil
	}
	switch c.Type {
	case v1beta1.HashTypeHeader:
		if len(c.Headers) == 0 {
			return fmt.Errorf("spec.traffic.consistentHash.headers must be non-empty when type=Header (MissingHashKey)")
		}
		if c.Cookie != nil {
			return fmt.Errorf("spec.traffic.consistentHash.cookie must not be set when type=Header (MultipleHashKeys)")
		}
		for i, h := range c.Headers {
			if h.Name == "" {
				return fmt.Errorf("spec.traffic.consistentHash.headers[%d].name is empty (MissingHashKey)", i)
			}
		}
	case v1beta1.HashTypeCookie:
		if c.Cookie == nil {
			return fmt.Errorf("spec.traffic.consistentHash.cookie is required when type=Cookie (MissingHashKey)")
		}
		if c.Cookie.Name == "" {
			return fmt.Errorf("spec.traffic.consistentHash.cookie.name is empty (MissingHashKey)")
		}
		if len(c.Headers) > 0 {
			return fmt.Errorf("spec.traffic.consistentHash.headers must not be set when type=Cookie (MultipleHashKeys)")
		}
	case v1beta1.HashTypeSourceIP:
		if c.Cookie != nil {
			return fmt.Errorf("spec.traffic.consistentHash.cookie must not be set when type=SourceIP (MultipleHashKeys)")
		}
		if len(c.Headers) > 0 {
			return fmt.Errorf("spec.traffic.consistentHash.headers must not be set when type=SourceIP (MultipleHashKeys)")
		}
	default:
		// CRD enum tag should reject this before reaching the webhook;
		// keep a defensive check in case CRD validation is bypassed.
		return fmt.Errorf("spec.traffic.consistentHash.type=%q is not a valid HashType", string(c.Type))
	}
	return nil
}

// validateEndpointOverride enforces the Headers requirement when
// Type=Header.
func validateEndpointOverride(e *v1beta1.EndpointOverrideSpec) error {
	if e == nil {
		return nil
	}
	switch e.Type {
	case v1beta1.EndpointOverrideTypeHeader:
		if len(e.Headers) == 0 {
			return fmt.Errorf("spec.traffic.endpointOverride.headers must be non-empty when type=Header (MissingEndpointOverrideKey)")
		}
		for i, h := range e.Headers {
			if h.Name == "" {
				return fmt.Errorf("spec.traffic.endpointOverride.headers[%d].name is empty (MissingEndpointOverrideKey)", i)
			}
		}
	case v1beta1.EndpointOverrideTypeMetadata:
		// Metadata variant is reserved for future use; no required
		// sub-fields in alpha.
	default:
		return fmt.Errorf("spec.traffic.endpointOverride.type=%q is not a valid EndpointOverrideType", string(e.Type))
	}
	return nil
}
