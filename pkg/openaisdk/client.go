package openaisdk

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/openaisdk/option"
	"os"
)

type Client struct {
	Options []option.RequestOption

	Projects *ProjectService
}

// NewClient generates a new client with the default option read from the
// environment (OPENAI_API_KEY, OPENAI_ORG_ID, OPENAI_PROJECT_ID). The option
// passed in as arguments are applied after these default arguments, and all option
// will be passed down to the services and requests that this client makes.
func NewClient(opts ...option.RequestOption) (r *Client) {
	defaults := []option.RequestOption{option.WithEnvironmentProduction()}
	if o, ok := os.LookupEnv("OPENAI_API_KEY"); ok {
		defaults = append(defaults, option.WithAPIKey(o))
	}
	if o, ok := os.LookupEnv("OPENAI_ORG_ID"); ok {
		defaults = append(defaults, option.WithOrganization(o))
	}
	if o, ok := os.LookupEnv("OPENAI_PROJECT_ID"); ok {
		defaults = append(defaults, option.WithProject(o))
	}
	opts = append(defaults, opts...)

	r = &Client{Options: opts}

	r.Projects = NewProjectService(opts...)

	return
}
