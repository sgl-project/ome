package services

import (
	"bytes"
	"fmt"
	"net/url"
	"text/template"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
)

// DefaultPathService implements PathService interface
type DefaultPathService struct{}

// NewPathService creates a new path service
func NewPathService() interfaces.PathService {
	return &DefaultPathService{}
}

type PathTemplateValues struct {
	Name      string
	Namespace string
}

// GenerateUrlPath generates the path using the pathTemplate configured in IngressConfig
func (p *DefaultPathService) GenerateUrlPath(name string, namespace string, ingressConfig *controllerconfig.IngressConfig) (string, error) {
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

	// Validate generated URL
	parsedURL, err := url.ParseRequestURI(buf.String())
	if err != nil {
		return "", fmt.Errorf("invalid url %q: %w", buf.String(), err)
	}

	if parsedURL.Scheme != "" || parsedURL.Host != "" {
		return "", fmt.Errorf("invalid url path %q: contains either a scheme or a host", buf.String())
	}

	return parsedURL.Path, nil
}
