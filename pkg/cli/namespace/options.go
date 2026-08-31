// Package namespace resolves the distinct namespaces used by kubectl-ome.
package namespace

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	DefaultOMENamespace     = "ome"
	DefaultAlfredConfigName = "alfred-config"
	DefaultAlfredConfigKey  = "config.yaml"
)

// Options holds namespace settings shared by CLI commands.
type Options struct {
	OMENamespace     string
	AlfredNamespace  string
	AlfredConfigName string
	AlfredConfigKey  string

	omeNamespaceExplicit     bool
	alfredNamespaceExplicit  bool
	alfredConfigNameExplicit bool
	alfredConfigKeyExplicit  bool
}

// Resolved identifies the workload and OME control-plane namespaces.
type Resolved struct {
	WorkloadNamespace string
	OMENamespace      string
	AlfredNamespace   string
	AlfredConfigName  string
	AlfredConfigKey   string
}

// NewOptions returns namespace options with CLI-compatible defaults.
func NewOptions() *Options {
	return &Options{
		OMENamespace:     DefaultOMENamespace,
		AlfredConfigName: DefaultAlfredConfigName,
		AlfredConfigKey:  DefaultAlfredConfigKey,
	}
}

// AddFlags binds shared namespace settings to flags.
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	o.AddOMEFlags(flags)
	o.AddAlfredFlags(flags)
}

// AddOMEFlags binds the OME control-plane namespace setting to flags.
func (o *Options) AddOMEFlags(flags *pflag.FlagSet) {
	if o.OMENamespace == "" && !o.omeNamespaceExplicit {
		o.OMENamespace = DefaultOMENamespace
	}
	flags.Var(&trackedStringValue{target: &o.OMENamespace, explicit: &o.omeNamespaceExplicit},
		"ome-namespace", "Namespace where the OME control plane is installed")
}

// AddAlfredFlags binds Alfred's namespace and configuration settings to flags.
func (o *Options) AddAlfredFlags(flags *pflag.FlagSet) {
	if o.AlfredConfigName == "" && !o.alfredConfigNameExplicit {
		o.AlfredConfigName = DefaultAlfredConfigName
	}
	if o.AlfredConfigKey == "" && !o.alfredConfigKeyExplicit {
		o.AlfredConfigKey = DefaultAlfredConfigKey
	}
	flags.Var(&trackedStringValue{target: &o.AlfredNamespace, explicit: &o.alfredNamespaceExplicit},
		"alfred-namespace", "Namespace where Alfred is installed (defaults to --ome-namespace)")
	flags.Var(&trackedStringValue{target: &o.AlfredConfigName, explicit: &o.alfredConfigNameExplicit},
		"alfred-config-name", "Name of Alfred's configuration ConfigMap")
	flags.Var(&trackedStringValue{target: &o.AlfredConfigKey, explicit: &o.alfredConfigKeyExplicit},
		"alfred-config-key", "Key inside Alfred's configuration ConfigMap")
}

// Resolve combines and validates the kubectl workload namespace with
// control-plane options. It never returns an empty namespace.
func (o Options) Resolve(workloadNamespace string) (Resolved, error) {
	omeNamespace, err := effectiveValue(
		o.OMENamespace, o.omeNamespaceExplicit, DefaultOMENamespace, "OME namespace",
	)
	if err != nil {
		return Resolved{}, err
	}
	alfredNamespace, err := effectiveValue(
		o.AlfredNamespace, o.alfredNamespaceExplicit, omeNamespace, "Alfred namespace",
	)
	if err != nil {
		return Resolved{}, err
	}
	alfredConfigName, err := effectiveValue(
		o.AlfredConfigName, o.alfredConfigNameExplicit, DefaultAlfredConfigName, "Alfred config name",
	)
	if err != nil {
		return Resolved{}, err
	}
	alfredConfigKey, err := effectiveValue(
		o.AlfredConfigKey, o.alfredConfigKeyExplicit, DefaultAlfredConfigKey, "Alfred config key",
	)
	if err != nil {
		return Resolved{}, err
	}

	if err := validateDNS1123Label("workload namespace", workloadNamespace); err != nil {
		return Resolved{}, err
	}
	if err := validateDNS1123Label("OME namespace", omeNamespace); err != nil {
		return Resolved{}, err
	}
	if err := validateDNS1123Label("Alfred namespace", alfredNamespace); err != nil {
		return Resolved{}, err
	}
	if problems := validation.IsDNS1123Subdomain(alfredConfigName); len(problems) > 0 {
		return Resolved{}, invalidValue("Alfred config name", alfredConfigName, problems)
	}
	if problems := validation.IsConfigMapKey(alfredConfigKey); len(problems) > 0 {
		return Resolved{}, invalidValue("Alfred config key", alfredConfigKey, problems)
	}

	return Resolved{
		WorkloadNamespace: workloadNamespace,
		OMENamespace:      omeNamespace,
		AlfredNamespace:   alfredNamespace,
		AlfredConfigName:  alfredConfigName,
		AlfredConfigKey:   alfredConfigKey,
	}, nil
}

func effectiveValue(value string, explicit bool, fallback, label string) (string, error) {
	if value != "" {
		return value, nil
	}
	if explicit {
		return "", fmt.Errorf("%s must not be empty", label)
	}
	return fallback, nil
}

func validateDNS1123Label(label, value string) error {
	if problems := validation.IsDNS1123Label(value); len(problems) > 0 {
		return invalidValue(label, value, problems)
	}
	return nil
}

func invalidValue(label, value string, problems []string) error {
	return fmt.Errorf("%s %q is invalid: %s", label, value, strings.Join(problems, "; "))
}

type trackedStringValue struct {
	target   *string
	explicit *bool
}

func (v *trackedStringValue) Set(value string) error {
	*v.target = value
	*v.explicit = true
	return nil
}

func (v *trackedStringValue) String() string {
	return *v.target
}

func (*trackedStringValue) Type() string {
	return "string"
}
