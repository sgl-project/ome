package testing

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	schedulerpluginsv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
)

func NewClientBuilder(addToSchemes ...func(s *runtime.Scheme) error) *fake.ClientBuilder {
	scm := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scm))
	utilruntime.Must(omev1beta1.AddToScheme(scm))
	utilruntime.Must(jobsetv1alpha2.AddToScheme(scm))
	utilruntime.Must(schedulerpluginsv1alpha1.AddToScheme(scm))
	for i := range addToSchemes {
		utilruntime.Must(addToSchemes[i](scm))
	}
	return fake.NewClientBuilder().
		WithScheme(scm)
}

type builderIndexer struct {
	*fake.ClientBuilder
}

var _ client.FieldIndexer = (*builderIndexer)(nil)

func (b *builderIndexer) IndexField(_ context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	if obj == nil || field == "" || extractValue == nil {
		return fmt.Errorf("error from test indexer")
	}
	b.ClientBuilder = b.ClientBuilder.WithIndex(obj, field, extractValue)
	return nil
}

func AsIndex(builder *fake.ClientBuilder) client.FieldIndexer {
	return &builderIndexer{ClientBuilder: builder}
}
