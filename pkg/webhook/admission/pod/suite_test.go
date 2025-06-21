package pod

import (
	"os"
	"testing"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pkgtest "github.com/sgl-project/ome/pkg/testing"
)

var cfg *rest.Config
var c client.Client
var clientset kubernetes.Interface

func TestMain(m *testing.M) {
	t := pkgtest.SetupEnvTest()

	err := v1beta1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err, "Failed to add v1beta1 to scheme")
	}

	if cfg, err = t.Start(); err != nil {
		klog.Error(err, "Failed to start testing panel")
	}

	if c, err = client.New(cfg, client.Options{Scheme: scheme.Scheme}); err != nil {
		klog.Error(err, "Failed to start client")
	}

	if clientset, err = kubernetes.NewForConfig(cfg); err != nil {
		klog.Error(err, "Failed to create clientset")
	}

	code := m.Run()
	_ = t.Stop()
	os.Exit(code)
}
