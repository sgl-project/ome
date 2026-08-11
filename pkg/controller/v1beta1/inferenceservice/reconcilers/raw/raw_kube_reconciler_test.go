package raw

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
)

// ingressConfigMap builds the inferenceservice-config ConfigMap the ingress
// config loader reads, with the given urlScheme (empty ⇒ omitted so the loader
// applies its own default).
func ingressConfigMap(urlScheme string) *corev1.ConfigMap {
	ingress := `{"ingressGateway":"knative-serving/knative-ingress-gateway","ingressService":"istio-ingressgateway.istio-system.svc.cluster.local","ingressDomain":"example.com","domainTemplate":"{{.Name}}.{{.Namespace}}.{{.IngressDomain}}"`
	if urlScheme != "" {
		ingress += `,"urlScheme":"` + urlScheme + `"`
	}
	ingress += `}`
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.InferenceServiceConfigMapName,
			Namespace: constants.OMENamespace,
		},
		Data: map[string]string{
			controllerconfig.IngressConfigKeyName: ingress,
		},
	}
}

// TestCreateRawURL_HonorsUrlScheme guards that the raw
// per-component status URL must carry the configured urlScheme, not a hardcoded
// http.
func TestCreateRawURL_HonorsUrlScheme(t *testing.T) {
	for _, tc := range []struct {
		name       string
		urlScheme  string
		wantScheme string
	}{
		{name: "https is honored", urlScheme: "https", wantScheme: "https"},
		{name: "http is honored", urlScheme: "http", wantScheme: "http"},
		{name: "empty falls back to loader default", urlScheme: "", wantScheme: controllerconfig.DefaultUrlScheme},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset(ingressConfigMap(tc.urlScheme))
			meta := metav1.ObjectMeta{Name: "my-isvc", Namespace: "my-ns"}

			url, err := createRawURL(clientset, meta)
			require.NoError(t, err)
			assert.Equal(t, tc.wantScheme, url.Scheme)
			assert.Equal(t, "my-isvc.my-ns.example.com", url.Host)
		})
	}
}
