// Package factory hands commands their API clients. Commands depend on the
// Factory interface only, so tests substitute Static and the real
// implementation stays in one place.
package factory

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/client/clientset/versioned"
)

type Factory interface {
	// RESTConfig returns the resolved client-go REST config.
	RESTConfig() (*rest.Config, error)
	// KubeClient returns a core Kubernetes clientset (pods, events, logs).
	KubeClient() (kubernetes.Interface, error)
	// OMEClient returns the generated OME typed clientset.
	OMEClient() (versioned.Interface, error)
	// RuntimeClient returns a lazily constructed controller-runtime client
	// (scheme: client-go core + ome.io/v1beta1) for commands that reuse
	// operator libraries such as pkg/runtimeselector.
	RuntimeClient() (ctrlclient.Client, error)
	// Namespace returns the effective namespace and whether the user set it
	// explicitly (flag or kubeconfig context override).
	Namespace() (string, bool, error)
}

func New(flags *genericclioptions.ConfigFlags) Factory {
	return &defaultFactory{flags: flags}
}

type defaultFactory struct {
	flags *genericclioptions.ConfigFlags

	mu      sync.Mutex
	rest    *rest.Config
	kube    kubernetes.Interface
	ome     versioned.Interface
	runtime ctrlclient.Client
}

func (f *defaultFactory) RESTConfig() (*rest.Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rest != nil {
		return f.rest, nil
	}
	cfg, err := f.flags.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	// CLI bursts many small reads (status fans out to pods/events); avoid
	// client-side throttling stalls.
	cfg.QPS = 50
	cfg.Burst = 100
	f.rest = cfg
	return cfg, nil
}

func (f *defaultFactory) KubeClient() (kubernetes.Interface, error) {
	f.mu.Lock()
	if f.kube != nil {
		defer f.mu.Unlock()
		return f.kube, nil
	}
	f.mu.Unlock()
	cfg, err := f.RESTConfig()
	if err != nil {
		return nil, err
	}
	c, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.kube = c
	return c, nil
}

func (f *defaultFactory) OMEClient() (versioned.Interface, error) {
	f.mu.Lock()
	if f.ome != nil {
		defer f.mu.Unlock()
		return f.ome, nil
	}
	f.mu.Unlock()
	cfg, err := f.RESTConfig()
	if err != nil {
		return nil, err
	}
	c, err := versioned.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ome = c
	return c, nil
}

func (f *defaultFactory) RuntimeClient() (ctrlclient.Client, error) {
	f.mu.Lock()
	if f.runtime != nil {
		defer f.mu.Unlock()
		return f.runtime, nil
	}
	f.mu.Unlock()
	cfg, err := f.RESTConfig()
	if err != nil {
		return nil, err
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1beta1.AddToScheme(scheme))
	c, err := ctrlclient.New(cfg, ctrlclient.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runtime = c
	return c, nil
}

func (f *defaultFactory) Namespace() (string, bool, error) {
	return f.flags.ToRawKubeConfigLoader().Namespace()
}
