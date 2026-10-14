package v1beta1

import (
	"bytes"
	"encoding/json"
)

// UnmarshalJSON preserves null affinity as an invalid value rather than an
// unconstrained omission, including when admission webhooks are unavailable.
// A clusterAffinity object, which this version cannot read, is held as invalid
// instead of failing to decode the whole service.
func (p *PlacementSpec) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	_, affinityPresent := fields["clusterAffinity"]
	if isJSONObject(fields["clusterAffinity"]) {
		delete(fields, "clusterAffinity")
		readable, err := json.Marshal(fields)
		if err != nil {
			return err
		}
		data = readable
	}
	type plain PlacementSpec
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if affinityPresent && decoded.ClusterAffinity == nil {
		decoded.ClusterAffinity = []ClusterAffinityTerm{}
	}
	if _, exists := fields["componentAffinity"]; exists && decoded.ComponentAffinity == nil {
		decoded.ComponentAffinity = &ComponentClusterAffinity{}
	}
	*p = PlacementSpec(decoded)
	return nil
}

// isJSONObject reports whether a raw JSON value is an object.
func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	return len(trimmed) > 0 && trimmed[0] == '{'
}

// MarshalJSON keeps explicit invalid restrictions visible across a read/write
// round trip, so clearing them requires an intentional edit of the API object.
func (p PlacementSpec) MarshalJSON() ([]byte, error) {
	type plain PlacementSpec
	data, err := json.Marshal(plain(p))
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if p.ClusterAffinity != nil && len(p.ClusterAffinity) == 0 {
		fields["clusterAffinity"] = json.RawMessage(`[]`)
	}
	return json.Marshal(fields)
}
