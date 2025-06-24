package integration_tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/controllerconfig"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/benchmark"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	testEnv   *envtest.Environment
	k8sClient client.Client
	ctx       context.Context
	cancel    context.CancelFunc
)

var _ = BeforeSuite(func() {
	var err error
	ctx, cancel = context.WithCancel(context.TODO())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "full", "ome.io_benchmarkjobs.yaml")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = v1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	mgr, err := manager.New(cfg, manager.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).NotTo(HaveOccurred())

	clientset, err := kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())

	ctrl.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))
	reconciler := benchmark.BenchmarkJobReconciler{
		Client:    mgr.GetClient(),
		Clientset: clientset,
		Scheme:    mgr.GetScheme(),
	}
	err = reconciler.SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err := mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("BenchmarkJob Controller e2e Test", func() {
	It("reconcile should add finalizer, K8Job with genai-bench with correct args", func() {
		imageName := "genai-bench:0.1.132"
		apiBase := "http://localhost:8080/"
		apiFormat := "openai"
		ossNamespace := "namespace_name"
		bucket := "bucket_name"
		prefix := "object_path"
		ossPath := fmt.Sprintf("oci://n/%s/b/%s/o/%s", ossNamespace, bucket, prefix)

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: constants.OMENamespace,
			},
		}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())

		componentConfig, err := json.Marshal(controllerconfig.BenchmarkJobConfig{
			PodConfig: controllerconfig.PodConfig{
				CPURequest:    "1",
				CPULimit:      "1",
				MemoryLimit:   "1",
				MemoryRequest: "1",
				Image:         imageName,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: constants.OMENamespace,
				Name:      constants.BenchmarkJobConfigMapName,
			},
			Data: map[string]string{
				controllerconfig.BenchmarkJobConfigName: string(componentConfig),
			},
		}
		Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

		job := &v1beta1.BenchmarkJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-benchmark",
				Namespace: ns.Name,
			},
			Spec: v1beta1.BenchmarkJobSpec{
				Task:                    "text-to-text",
				MaxRequestsPerIteration: intPtr(10),
				MaxTimePerIteration:     intPtr(10),
				OutputLocation: &v1beta1.StorageSpec{
					StorageUri: strPtr(ossPath),
				},
				Endpoint: v1beta1.EndpointSpec{
					Endpoint: &v1beta1.Endpoint{
						URL:       apiBase,
						APIFormat: apiFormat,
					},
				},
			},
		}

		Expect(k8sClient.Create(ctx, job)).To(Succeed())

		By("waiting for BenchmarkJob to be updated with finalizer")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(job), job)
			return err == nil && controllerutil.ContainsFinalizer(job, "benchmarkjob.finalizers")
		}, time.Second*100, time.Second).Should(BeTrue())

		k8Job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-benchmark",
				Namespace: ns.Name,
			},
		}
		By("waiting for BenchmarkJob to create Job")
		Eventually(func() bool {
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(k8Job), k8Job)
			return err == nil
		}, time.Second*100, time.Second).Should(BeTrue())

		podSpec := k8Job.Spec.Template.Spec
		Expect(len(podSpec.Containers)).To(Equal(1))
		containerSpec := podSpec.Containers[0]
		Expect(containerSpec.Image).To(Equal(imageName))
		Expect(len(containerSpec.Env)).To(Equal(1))
		Expect(containerSpec.Env[0].Name).To(Equal("ENABLE_UI"))
		Expect(containerSpec.Env[0].Value).To(Equal("false"))

		expectArgs := map[string]string{
			"--api-base":             apiBase,
			"--api-backend":          apiFormat,
			"--max-time-per-run":     "10",
			"--max-requests-per-run": "10",
			"--namespace":            ossNamespace,
			"--bucket":               bucket,
			"--prefix":               prefix,
		}
		for key, expectedValue := range expectArgs {
			idx := -1
			for i, arg := range containerSpec.Args {
				if arg == key && i+1 < len(containerSpec.Args) {
					idx = i + 1
					break
				}
			}
			Expect(idx).ToNot(Equal(-1), "Couldn't find %s ", key)
			Expect(containerSpec.Args[idx]).To(Equal(expectedValue), "Unexpected value for %s", key)
		}
	})
})

func intPtr(i int) *int       { return &i }
func strPtr(s string) *string { return &s }
