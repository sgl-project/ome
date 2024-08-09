package vars

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFallback_Resolve(t *testing.T) {
	resolver := &FallbackResolver{
		config: FallbackResolverConfig{
			Ad:     "foobarAd",
			Region: "foobarRegion",
			Realm:  "foobarRealm",
		},
	}

	t.Run("happy case", func(t *testing.T) {
		region, err := resolver.Resolve(Region)
		assert.NoError(t, err)
		assert.Equal(t, "foobarRegion", region, "region should be resolved")

		ad, err := resolver.Resolve(Ad)
		assert.NoError(t, err)
		assert.Equal(t, "foobarAd", ad, "ad should be resolved")

		realm, err := resolver.Resolve(Realm)
		assert.NoError(t, err)
		assert.Equal(t, "foobarRealm", realm, "realm should be resolved")
	})

	t.Run("empty var", func(t *testing.T) {
		resolver.config.Region = ""
		assert.Error(t, resolver.config.Validate())

		region, err := resolver.Resolve(Region)
		assert.Equal(t, "", region)
		assert.NoError(t, err)
	})

	t.Run("unknown env var", func(t *testing.T) {
		region, err := resolver.Resolve(MustNewVar("REGION-X", false))
		assert.Equal(t, "", region)
		assert.Error(t, err)
	})
}
