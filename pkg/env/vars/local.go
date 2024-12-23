package vars

import (
	"fmt"
	"strings"

	"github.com/spf13/afero"
)

var (
	ResourceCompartmentId = MustNewVar("resourceCompartmentID", true)
)

var (
	// Default local vars' file paths resolved by local resolver.
	localVarsToFilePaths = map[Var]string{
		Ad:                    "/etc/availability-domain",
		Region:                "/etc/region",
		Realm:                 "/etc/identity-realm",
		Hostclass:             "/etc/hostclass",
		ResourceCompartmentId: "/etc/resource-compartment-id",
	}
)

type LocalResolverConfig struct {
	// AdditionalVars allow for local resolver to substitute variables
	// using custom defined files and their contents.
	//
	// N.B. not using maps since Viper lower-cases the map keys
	AdditionalVars []LocalAdditionalVar `mapstructure:"additional_vars"`
}

type LocalAdditionalVar struct {
	Name     string `mapstructure:"name"`
	IsOcid   bool   `mapstructure:"is_ocid"`
	FilePath string `mapstructure:"file_path"`
}

// constructAddlVars returns a map of Var -> filePath
// collecting all additional vars specified in the config.
func (c LocalResolverConfig) constructAddlVars() (map[Var]string, error) {
	if len(c.AdditionalVars) == 0 {
		return nil, nil
	}

	// TODO(achebatu): BUG: var _names_ should be unique, but instead we're differentiating over vars.
	//  Var("x", isOcid=true) != Var("x", isOcid=false)
	result := make(map[Var]string, len(c.AdditionalVars))
	for _, conf := range c.AdditionalVars {
		varName := conf.Name
		v, err := NewVar(varName, conf.IsOcid)
		if err != nil {
			return nil, err
		}

		if _, ok := result[v]; ok {
			return nil, fmt.Errorf("local additional var is already defined: %v", v)
		}

		if _, ok := localVarsToFilePaths[v]; ok {
			return nil, fmt.Errorf("can't redefine var: %v", v)
		}

		result[v] = conf.FilePath
	}
	return result, nil
}

// LocalResolver represents a var resolver that uses filesystem.
type LocalResolver struct {
	varsToFilePaths map[Var]string
	fs              afero.Fs
}

func NewLocalResolver(config LocalResolverConfig, fs afero.Fs) (*LocalResolver, error) {
	varsToFilePaths, err := config.constructAddlVars()
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &LocalResolver{
		varsToFilePaths: varsToFilePaths,
		fs:              fs,
	}, nil
}

func (o LocalResolver) Resolve(v Var) (string, error) {
	filePath, ok := localVarsToFilePaths[v]
	if !ok {
		filePath, ok = o.varsToFilePaths[v]
	}
	if !ok {
		return "", fmt.Errorf("local can't resolve var %v", v)
	}

	result, err := o.resolve(filePath)
	if err != nil {
		return "", fmt.Errorf("local can't resolve var %v: %w", v, err)
	}

	if v.IsOcid() {
		return escapeOCID(result), nil
	}

	if v.IsHostClassName() {
		return strings.ToLower(result), nil
	}

	return result, nil
}

func (o LocalResolver) CanResolve() []Var {
	result := []Var{
		Region,
		Realm,
		Ad,
		Hostclass,
		ResourceCompartmentId,
	}

	for v := range o.varsToFilePaths {
		result = append(result, v)
	}

	return result
}

func (o LocalResolver) resolve(filePath string) (string, error) {
	resultBytes, err := afero.ReadFile(o.fs, filePath)
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(string(resultBytes))
	if result == "" {
		return "", fmt.Errorf("empty file: %s", filePath)
	}

	return result, nil
}

var _ Resolver = &LocalResolver{}
