package imds

import "encoding/json"

const (
	tagNamespaceHostAccess = "host-access"
	tagKeySubclass         = "subclass"
)

// DefinedTags encapsulates the type of defined tags on an instance.
type DefinedTags map[string]map[string]string

// instanceResultRaw encapsulates the shape of the raw JSON response
// from /instance endpoint.
type instanceResultRaw struct {
	CompartmentId string `json:"compartmentId"`
	Shape         string `json:"shape"`

	RegionInfo struct {
		Realm  string `json:"realmKey"`
		Region string `json:"regionIdentifier"`
	} `json:"regionInfo"`

	Metadata struct {
		Hostclass string `json:"hostclass"`

		// TODO(achebatu): deprecate and use DefinedTags only.
		// Check instance metadata service availability
		Hostsubclass string `json:"hostsubclass,omitempty"`
	} `json:"metadata"`

	DefinedTags DefinedTags `json:"definedTags,omitempty"`
}

// parseInstanceResult parses the raw /instance JSON payload and converts it into instanceResult.
func parseInstanceResult(jsonBytes []byte) (instanceResult, error) {
	var jsonValue instanceResultRaw
	if err := json.Unmarshal(jsonBytes, &jsonValue); err != nil {
		return instanceResult{}, err
	}

	return jsonValue.asInstanceResult()
}

// asInstanceResult maps the values of instanceResultRaw into instanceResult.
//
// Most fields are mapped as-is, but the HostSubclass value is reconciled the following way
// (for backward compatibility until ECAR is approved):
//   - use definedTags.host-access.subclass, if it exists and is not empty;
//   - use metadata.hostsubclass, if it exists and is not empty;
//   - use "" otherwise.
func (j instanceResultRaw) asInstanceResult() (instanceResult, error) {
	hostsubclass := j.DefinedTags[tagNamespaceHostAccess][tagKeySubclass]
	if hostsubclass == "" {
		hostsubclass = j.Metadata.Hostsubclass // fall back (temporarily)
	}

	return instanceResult{
		Realm:         j.RegionInfo.Realm,
		Region:        j.RegionInfo.Region,
		CompartmentId: j.CompartmentId,
		Hostclass:     j.Metadata.Hostclass,
		Shape:         j.Shape,
		HostSubclass:  hostsubclass,
	}, nil
}
