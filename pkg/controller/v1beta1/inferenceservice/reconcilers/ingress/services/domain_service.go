package services

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"knative.dev/pkg/network"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/inferenceservice/reconcilers/ingress/interfaces"
)

var log = logf.Log.WithName("DomainService")

// DefaultDomainService implements DomainService interface
type DefaultDomainService struct{}

// NewDomainService creates a new domain service
func NewDomainService() interfaces.DomainService {
	return &DefaultDomainService{}
}

type DomainTemplateValues struct {
	Name          string
	Namespace     string
	IngressDomain string
	Annotations   map[string]string
	Labels        map[string]string
}

// GenerateDomainName generates domain name using template configured in IngressConfig
func (d *DefaultDomainService) GenerateDomainName(name string, obj interface{}, ingressConfig *controllerconfig.IngressConfig) (string, error) {
	var objMeta metav1.ObjectMeta

	switch v := obj.(type) {
	case *v1beta1.InferenceService:
		objMeta = v.ObjectMeta
	case metav1.ObjectMeta:
		objMeta = v
	default:
		return "", fmt.Errorf("unsupported object type for domain generation")
	}

	values := DomainTemplateValues{
		Name:          name,
		Namespace:     objMeta.Namespace,
		IngressDomain: ingressConfig.IngressDomain,
		Annotations:   objMeta.Annotations,
		Labels:        objMeta.Labels,
	}

	tpl, err := template.New("domain-template").Parse(ingressConfig.DomainTemplate)
	if err != nil {
		return "", err
	}

	buf := bytes.Buffer{}
	if err := tpl.Execute(&buf, values); err != nil {
		return "", fmt.Errorf("error rendering the domain template: %w", err)
	}

	urlErrs := validation.IsFullyQualifiedDomainName(field.NewPath("url"), buf.String())
	if urlErrs != nil {
		return "", fmt.Errorf("invalid domain name %q: %w", buf.String(), urlErrs.ToAggregate())
	}

	return buf.String(), nil
}

// GenerateInternalDomainName generates internal domain name using cluster domain
func (d *DefaultDomainService) GenerateInternalDomainName(name string, obj interface{}, ingressConfig *controllerconfig.IngressConfig) (string, error) {
	var objMeta metav1.ObjectMeta

	switch v := obj.(type) {
	case *v1beta1.InferenceService:
		objMeta = v.ObjectMeta
	case metav1.ObjectMeta:
		objMeta = v
	default:
		return "", fmt.Errorf("unsupported object type for domain generation")
	}

	values := DomainTemplateValues{
		Name:          name,
		Namespace:     objMeta.Namespace,
		IngressDomain: network.GetClusterDomainName(),
		Annotations:   objMeta.Annotations,
		Labels:        objMeta.Labels,
	}

	tpl, err := template.New("domain-template").Parse(ingressConfig.DomainTemplate)
	if err != nil {
		return "", err
	}

	buf := bytes.Buffer{}
	if err := tpl.Execute(&buf, values); err != nil {
		return "", fmt.Errorf("error rendering the domain template: %w", err)
	}

	urlErrs := validation.IsFullyQualifiedDomainName(field.NewPath("url"), buf.String())
	if urlErrs != nil {
		return "", fmt.Errorf("invalid domain name %q: %w", buf.String(), urlErrs.ToAggregate())
	}

	return buf.String(), nil
}

// GetAdditionalHosts generates additional hostnames for an InferenceService
func (d *DefaultDomainService) GetAdditionalHosts(domainList *[]string, serviceHost string, config *controllerconfig.IngressConfig) *[]string {
	additionalHosts := &[]string{}
	subdomain := ""

	if domainList != nil && len(*domainList) != 0 {
		for _, domain := range *domainList {
			res, found := strings.CutSuffix(serviceHost, domain)
			if found {
				subdomain = res
				break
			}
		}
	}

	if len(subdomain) != 0 && config.AdditionalIngressDomains != nil && len(*config.AdditionalIngressDomains) > 0 {
		deduplicateMap := make(map[string]bool, len(*config.AdditionalIngressDomains))
		for _, domain := range *config.AdditionalIngressDomains {
			if !deduplicateMap[domain] {
				host := fmt.Sprintf("%s%s", subdomain, domain)
				if err := validation.IsDNS1123Subdomain(host); len(err) > 0 {
					log.Error(fmt.Errorf("the domain name %s in the additionalIngressDomains is not valid", domain),
						"Failed to get the valid host name")
					continue
				}
				*additionalHosts = append(*additionalHosts, host)
				deduplicateMap[domain] = true
			}
		}
	}

	return additionalHosts
}
