package ingress

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	"net/url"
)

type PathTemplateValues struct {
	Name      string
	Namespace string
}

// GenerateUrlPath generates the path using the pathTemplate configured in IngressConfig
func GenerateUrlPath(name string, namespace string, ingressConfig *controllerconfig.IngressConfig) (string, error) {
	if ingressConfig.PathTemplate == "" {
		return "", nil
	}

	values := PathTemplateValues{
		Name:      name,
		Namespace: namespace,
	}
	tpl, err := template.New("url-template").Parse(ingressConfig.PathTemplate)
	if err != nil {
		return "", err
	}

	buf := bytes.Buffer{}
	if err := tpl.Execute(&buf, values); err != nil {
		return "", fmt.Errorf("error rendering the url template: %w", err)
	}

	// Validate generated URL. Use url.ParseRequestURI() instead of
	// apis.ParseURL(). The latter calls url.Parse() which allows pretty much anything.
	url, err := url.ParseRequestURI(buf.String())
	if err != nil {
		return "", fmt.Errorf("invalid url %q: %w", buf.String(), err)
	}

	if url.Scheme != "" || url.Host != "" {
		return "", fmt.Errorf("invalid url path %q: contains either a scheme or a host", buf.String())
	}

	return url.Path, nil
}
