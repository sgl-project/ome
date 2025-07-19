package testing

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	istioclientv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/kubernetes/scheme"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var log = logf.Log.WithName("TestingEnvSetup")

func SetupEnvTest() *envtest.Environment {
	t := &envtest.Environment{
		// The relative paths must be provided for each level of test nesting
		// This code should be illegal
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "..", "..", "..", "config", "crd", "ome.io_trainedmodels.yaml"),
			filepath.Join("..", "..", "..", "..", "..", "..", "test", "crds"),
			filepath.Join("..", "..", "..", "..", "config", "crd", "ome.io_trainedmodels.yaml"),
			filepath.Join("..", "..", "..", "..", "test", "crds"),
		},
		UseExistingCluster: proto.Bool(false),
	}

	if err := netv1.SchemeBuilder.AddToScheme(scheme.Scheme); err != nil {
		log.Error(err, "Failed to add networking v1 scheme")
	}

	if err := knservingv1.SchemeBuilder.AddToScheme(scheme.Scheme); err != nil {
		log.Error(err, "Failed to add knative serving scheme")
	}

	if err := istioclientv1beta1.SchemeBuilder.AddToScheme(scheme.Scheme); err != nil {
		log.Error(err, "Failed to add istio scheme")
	}
	return t
}

// StartTestManager adds recFn
func StartTestManager(ctx context.Context, mgr manager.Manager, g *gomega.GomegaWithT) *sync.WaitGroup {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		g.Expect(mgr.Start(ctx)).NotTo(gomega.HaveOccurred())
	}()
	return wg
}
