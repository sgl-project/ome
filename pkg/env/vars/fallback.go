package vars

import (
	"fmt"
)

type FallbackResolverConfig struct {
	Region string `mapstructure:"region"`
	Realm  string `mapstructure:"realm"`
	Ad     string `mapstructure:"ad"`
}

func (f FallbackResolverConfig) Validate() error {
	if f.Realm == "" {
		return fmt.Errorf("empty realm")
	}
	if f.Region == "" {
		return fmt.Errorf("empty region")
	}
	if f.Ad == "" {
		return fmt.Errorf("empty ad")
	}

	return nil
}

// FallbackResolver represents a var resolver that
// returns hardcoded values.
type FallbackResolver struct {
	config FallbackResolverConfig
}

func NewFallbackResolver(c FallbackResolverConfig) (FallbackResolver, error) {
	if err := c.Validate(); err != nil {
		return FallbackResolver{}, fmt.Errorf("invalid fallback resolver config: %w", err)
	}

	return FallbackResolver{config: c}, nil
}

func (f FallbackResolver) CanResolve() []Var {
	return []Var{
		Region,
		Realm,
		Ad,
	}
}

func (f FallbackResolver) Resolve(v Var) (string, error) {
	switch v {
	case Region:
		return f.config.Region, nil
	case Realm:
		return f.config.Realm, nil
	case Ad:
		return f.config.Ad, nil
	}

	return "", fmt.Errorf("fallback can't resolve var: %v", v)
}

var _ Resolver = FallbackResolver{}
