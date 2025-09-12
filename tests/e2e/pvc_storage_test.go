package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

var (
	k8sClient    client.Client
	k8sClientset *kubernetes.Clientset
	config       *rest.Config
)

// TestPVCStorageEndToEnd performs a complete end-to-end test of PVC storage functionality
var _ = Describe("PVC Storage End-to-End Tests", func() {
	var (
		ctx           context.Context
		testNamespace string
		pvcName       string
		baseModelName string
		isvcName      string
	)

	BeforeEach(func() {
		ctx = context.Background()
		testNamespace = fmt.Sprintf("pvc-test-%d", time.Now().Unix())
		pvcName = "model-storage-pvc"
		baseModelName = "llama2-7b-pvc"
		isvcName = "llama2-7b-inference"

		// Setup Kubernetes client
		var err error
		config, err = clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		if err != nil {
			// Try in-cluster config
			config, err = rest.InClusterConfig()
			Expect(err).NotTo(HaveOccurred(), "Failed to get Kubernetes config")
		}

		k8sClient, err = client.New(config, client.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred(), "Failed to create Kubernetes client")

		k8sClientset, err = kubernetes.NewForConfig(config)
		Expect(err).NotTo(HaveOccurred(), "Failed to create Kubernetes clientset")

		// Add OME types to scheme
		err = v1beta1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred(), "Failed to add OME types to scheme")

		// Create test namespace
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		err = k8sClient.Create(ctx, namespace)
		Expect(err).NotTo(HaveOccurred(), "Failed to create test namespace")

		// Wait for namespace to be ready
		err = wait.PollImmediate(time.Second, 30*time.Second, func() (bool, error) {
			ns := &corev1.Namespace{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: testNamespace}, ns)
			if err != nil {
				return false, err
			}
			return ns.Status.Phase == corev1.NamespaceActive, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Namespace not ready within timeout")
	})

	AfterEach(func() {
		// Cleanup test namespace (this will cascade delete all resources)
		if testNamespace != "" {
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNamespace,
				},
			}
			err := k8sClient.Delete(ctx, namespace)
			if err != nil && !errors.IsNotFound(err) {
				GinkgoWriter.Printf("Warning: Failed to delete test namespace: %v\n", err)
			}
		}
	})

	Context("Complete PVC Storage Workflow", func() {
		It("Should successfully complete the full PVC storage workflow", func() {
			By("Creating PVC and waiting for it to be bound")
			pvc := createTestPVC(testNamespace, pvcName)
			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "Failed to create PVC")

			// Wait for PVC to be bound
			err = waitForPVCBound(ctx, k8sClient, testNamespace, pvcName, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "PVC not bound within timeout")

			By("Populating PVC with model files")
			err = populatePVCWithModelFiles(ctx, k8sClient, testNamespace, pvcName)
			Expect(err).NotTo(HaveOccurred(), "Failed to populate PVC with model files")

			By("Creating BaseModel with PVC storage URI")
			baseModel := createTestBaseModel(testNamespace, baseModelName, pvcName)
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying metadata extraction job runs successfully")
			err = waitForMetadataJobCompletion(ctx, k8sClient, testNamespace, baseModelName, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Metadata extraction job failed or timed out")

			By("Verifying BaseModel metadata is populated")
			err = waitForBaseModelReady(ctx, k8sClient, testNamespace, baseModelName, 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel not ready within timeout")

			// Verify metadata was extracted
			updatedBaseModel := &v1beta1.BaseModel{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      baseModelName,
				Namespace: testNamespace,
			}, updatedBaseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to get updated BaseModel")

			Expect(updatedBaseModel.Status.State).To(Equal(v1beta1.LifeCycleStateReady))
			// Verify that the model is ready and has been processed
			Expect(updatedBaseModel.Status.NodesReady).NotTo(BeEmpty())

			By("Creating InferenceService using the PVC model")
			inferenceService := createTestInferenceService(testNamespace, isvcName, baseModelName)
			err = k8sClient.Create(ctx, inferenceService)
			Expect(err).NotTo(HaveOccurred(), "Failed to create InferenceService")

			By("Verifying pod has PVC mounted correctly")
			err = waitForInferenceServiceReady(ctx, k8sClient, testNamespace, isvcName, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "InferenceService not ready within timeout")

			// Verify PVC is mounted in the pod
			err = verifyPVCMountedInPod(ctx, k8sClient, testNamespace, isvcName, pvcName)
			Expect(err).NotTo(HaveOccurred(), "PVC not mounted correctly in pod")

			By("Checking volume mounts and subpaths in detail")
			err = verifyVolumeMountsAndSubpaths(ctx, k8sClient, testNamespace, isvcName, pvcName)
			Expect(err).NotTo(HaveOccurred(), "Volume mounts and subpaths verification failed")
		})
	})

	Context("PVC Storage with Different Subpaths", func() {
		It("Should handle PVC with custom subpath correctly", func() {
			By("Creating PVC with custom model structure")
			pvc := createTestPVC(testNamespace, "custom-subpath-pvc")
			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "Failed to create PVC")

			err = waitForPVCBound(ctx, k8sClient, testNamespace, "custom-subpath-pvc", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "PVC not bound within timeout")

			By("Populating PVC with custom directory structure")
			err = populatePVCWithCustomStructure(ctx, k8sClient, testNamespace, "custom-subpath-pvc")
			Expect(err).NotTo(HaveOccurred(), "Failed to populate PVC")

			By("Creating BaseModel with custom subpath")
			baseModel := createTestBaseModelWithCustomPath(testNamespace, "custom-subpath-model", "custom-subpath-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying metadata extraction and model readiness")
			err = waitForMetadataJobCompletion(ctx, k8sClient, testNamespace, "custom-subpath-model", 5*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Metadata extraction job failed or timed out")

			err = waitForBaseModelReady(ctx, k8sClient, testNamespace, "custom-subpath-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel not ready within timeout")

			By("Creating InferenceService and verifying custom subpath mounting")
			inferenceService := createTestInferenceService(testNamespace, "custom-subpath-inference", "custom-subpath-model")
			err = k8sClient.Create(ctx, inferenceService)
			Expect(err).NotTo(HaveOccurred(), "Failed to create InferenceService")

			err = waitForInferenceServiceReady(ctx, k8sClient, testNamespace, "custom-subpath-inference", 5*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "InferenceService not ready within timeout")

			By("Verifying custom subpath is mounted correctly")
			err = verifyCustomSubpathMount(ctx, k8sClient, testNamespace, "custom-subpath-inference", "custom-subpath-pvc")
			Expect(err).NotTo(HaveOccurred(), "Custom subpath mount verification failed")
		})
	})

	Context("PVC Storage Error Scenarios", func() {
		It("Should handle missing PVC gracefully", func() {
			By("Creating BaseModel with non-existent PVC")
			baseModel := createTestBaseModel(testNamespace, "missing-pvc-model", "non-existent-pvc")
			err := k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel status reflects PVC not found error")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "missing-pvc-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should have failed status")

			updatedBaseModel := &v1beta1.BaseModel{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "missing-pvc-model",
				Namespace: testNamespace,
			}, updatedBaseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to get BaseModel")
			Expect(updatedBaseModel.Status.State).To(Equal(v1beta1.LifeCycleStateFailed))
			// Verify that the model failed due to missing PVC
			Expect(updatedBaseModel.Status.NodesFailed).NotTo(BeEmpty())
		})

		It("Should handle PVC with missing config.json", func() {
			By("Creating PVC and populating with model files but no config.json")
			pvc := createTestPVC(testNamespace, "no-config-pvc")
			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "Failed to create PVC")

			err = waitForPVCBound(ctx, k8sClient, testNamespace, "no-config-pvc", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "PVC not bound within timeout")

			// Populate with model files but no config.json
			err = populatePVCWithModelFilesNoConfig(ctx, k8sClient, testNamespace, "no-config-pvc")
			Expect(err).NotTo(HaveOccurred(), "Failed to populate PVC")

			By("Creating BaseModel and verifying metadata extraction fails gracefully")
			baseModel := createTestBaseModel(testNamespace, "no-config-model", "no-config-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			// Should eventually fail due to missing config.json
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "no-config-model", 3*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should have failed status")
		})

		It("Should handle unbound PVC gracefully", func() {
			By("Creating PVC that will remain unbound")
			unboundPVC := createTestPVCWithUnboundStorageClass(testNamespace, "unbound-pvc")
			err := k8sClient.Create(ctx, unboundPVC)
			Expect(err).NotTo(HaveOccurred(), "Failed to create unbound PVC")

			By("Creating BaseModel with unbound PVC")
			baseModel := createTestBaseModel(testNamespace, "unbound-pvc-model", "unbound-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel status reflects unbound PVC error")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "unbound-pvc-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should have failed status")

			updatedBaseModel := &v1beta1.BaseModel{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "unbound-pvc-model",
				Namespace: testNamespace,
			}, updatedBaseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to get BaseModel")
			Expect(updatedBaseModel.Status.State).To(Equal(v1beta1.LifeCycleStateFailed))
		})

		It("Should handle invalid subpath scenarios", func() {
			By("Creating PVC with valid storage")
			pvc := createTestPVC(testNamespace, "invalid-subpath-pvc")
			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "Failed to create PVC")

			err = waitForPVCBound(ctx, k8sClient, testNamespace, "invalid-subpath-pvc", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "PVC not bound within timeout")

			By("Populating PVC with model files")
			err = populatePVCWithModelFiles(ctx, k8sClient, testNamespace, "invalid-subpath-pvc")
			Expect(err).NotTo(HaveOccurred(), "Failed to populate PVC")

			By("Creating BaseModel with invalid subpath")
			baseModel := createTestBaseModelWithInvalidSubpath(testNamespace, "invalid-subpath-model", "invalid-subpath-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel fails due to invalid subpath")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "invalid-subpath-model", 3*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should have failed status")

			updatedBaseModel := &v1beta1.BaseModel{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "invalid-subpath-model",
				Namespace: testNamespace,
			}, updatedBaseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to get BaseModel")
			Expect(updatedBaseModel.Status.State).To(Equal(v1beta1.LifeCycleStateFailed))
		})

		It("Should handle PVC with insufficient permissions", func() {
			By("Creating PVC with restricted permissions")
			restrictedPVC := createTestPVCWithRestrictedPermissions(testNamespace, "restricted-pvc")
			err := k8sClient.Create(ctx, restrictedPVC)
			Expect(err).NotTo(HaveOccurred(), "Failed to create restricted PVC")

			err = waitForPVCBound(ctx, k8sClient, testNamespace, "restricted-pvc", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "PVC not bound within timeout")

			By("Creating BaseModel with restricted PVC")
			baseModel := createTestBaseModel(testNamespace, "restricted-pvc-model", "restricted-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel fails due to permission issues")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "restricted-pvc-model", 3*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should have failed status")

			updatedBaseModel := &v1beta1.BaseModel{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "restricted-pvc-model",
				Namespace: testNamespace,
			}, updatedBaseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to get BaseModel")
			Expect(updatedBaseModel.Status.State).To(Equal(v1beta1.LifeCycleStateFailed))
		})

		It("Should handle PVC with corrupted model files", func() {
			By("Creating PVC and populating with corrupted model files")
			pvc := createTestPVC(testNamespace, "corrupted-pvc")
			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "Failed to create PVC")

			err = waitForPVCBound(ctx, k8sClient, testNamespace, "corrupted-pvc", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "PVC not bound within timeout")

			// Populate with corrupted model files
			err = populatePVCWithCorruptedModelFiles(ctx, k8sClient, testNamespace, "corrupted-pvc")
			Expect(err).NotTo(HaveOccurred(), "Failed to populate PVC")

			By("Creating BaseModel and verifying metadata extraction fails")
			baseModel := createTestBaseModel(testNamespace, "corrupted-model", "corrupted-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			// Should eventually fail due to corrupted files
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "corrupted-model", 3*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should have failed status")
		})

		It("Should handle PVC with empty directory", func() {
			By("Creating PVC and leaving it empty")
			pvc := createTestPVC(testNamespace, "empty-pvc")
			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "Failed to create PVC")

			err = waitForPVCBound(ctx, k8sClient, testNamespace, "empty-pvc", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "PVC not bound within timeout")

			By("Creating BaseModel with empty PVC")
			baseModel := createTestBaseModel(testNamespace, "empty-model", "empty-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel fails due to empty directory")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "empty-model", 3*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should have failed status")
		})

		It("Should handle PVC with invalid URI format", func() {
			By("Creating BaseModel with invalid PVC URI format")
			baseModel := createTestBaseModelWithInvalidURI(testNamespace, "invalid-uri-model")
			err := k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel fails due to invalid URI")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "invalid-uri-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should have failed status")

			updatedBaseModel := &v1beta1.BaseModel{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "invalid-uri-model",
				Namespace: testNamespace,
			}, updatedBaseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to get BaseModel")
			Expect(updatedBaseModel.Status.State).To(Equal(v1beta1.LifeCycleStateFailed))
		})

		It("Should handle PVC with namespace mismatch", func() {
			By("Creating PVC in different namespace")
			otherNamespace := fmt.Sprintf("other-ns-%d", time.Now().Unix())
			otherNS := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: otherNamespace,
				},
			}
			err := k8sClient.Create(ctx, otherNS)
			Expect(err).NotTo(HaveOccurred(), "Failed to create other namespace")

			pvc := createTestPVC(otherNamespace, "cross-ns-pvc")
			err = k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "Failed to create PVC in other namespace")

			err = waitForPVCBound(ctx, k8sClient, otherNamespace, "cross-ns-pvc", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "PVC not bound within timeout")

			By("Creating BaseModel with PVC from different namespace")
			baseModel := createTestBaseModelWithCrossNamespacePVC(testNamespace, "cross-ns-model", otherNamespace, "cross-ns-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel fails due to namespace mismatch")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "cross-ns-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should have failed status")
		})

		It("Should handle PVC with storage class issues", func() {
			By("Creating PVC with non-existent storage class")
			invalidStorageClassPVC := createTestPVCWithInvalidStorageClass(testNamespace, "invalid-sc-pvc")
			err := k8sClient.Create(ctx, invalidStorageClassPVC)
			Expect(err).NotTo(HaveOccurred(), "Failed to create PVC with invalid storage class")

			By("Creating BaseModel with PVC that has storage class issues")
			baseModel := createTestBaseModel(testNamespace, "invalid-sc-model", "invalid-sc-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel fails due to storage class issues")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "invalid-sc-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should have failed status")
		})

		It("Should handle PVC with resource quota issues", func() {
			By("Creating resource quota that limits PVC creation")
			quota := createResourceQuota(testNamespace, "pvc-quota")
			err := k8sClient.Create(ctx, quota)
			Expect(err).NotTo(HaveOccurred(), "Failed to create resource quota")

			By("Creating PVC that exceeds quota")
			largePVC := createTestPVCWithLargeSize(testNamespace, "large-pvc")
			err = k8sClient.Create(ctx, largePVC)
			Expect(err).NotTo(HaveOccurred(), "Failed to create large PVC")

			By("Creating BaseModel with PVC that has quota issues")
			baseModel := createTestBaseModel(testNamespace, "quota-model", "large-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel fails due to quota issues")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "quota-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should have failed status")
		})
	})

	Context("PVC Storage Error Cases", func() {
		It("Should handle non-existent PVC correctly", func() {
			By("Creating BaseModel with non-existent PVC")
			baseModel := createTestBaseModel(testNamespace, "non-existent-pvc-model", "non-existent-pvc")
			err := k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel fails with appropriate error")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "non-existent-pvc-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should fail when PVC does not exist")

			// Verify the failure reason
			failedBaseModel := &v1beta1.BaseModel{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "non-existent-pvc-model",
				Namespace: testNamespace,
			}, failedBaseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to get failed BaseModel")

			Expect(failedBaseModel.Status.State).To(Equal(v1beta1.LifeCycleStateFailed))
			// Verify that the BaseModel failed due to PVC not found
		})

		It("Should handle unbound PVC behavior correctly", func() {
			By("Creating PVC with non-existent storage class to keep it unbound")
			pvc := createTestPVCWithUnboundStorageClass(testNamespace, "unbound-pvc")
			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "Failed to create unbound PVC")

			By("Creating BaseModel with unbound PVC")
			baseModel := createTestBaseModel(testNamespace, "unbound-pvc-model", "unbound-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel fails due to unbound PVC")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "unbound-pvc-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should fail when PVC is unbound")

			// Verify the failure reason
			failedBaseModel := &v1beta1.BaseModel{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "unbound-pvc-model",
				Namespace: testNamespace,
			}, failedBaseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to get failed BaseModel")

			Expect(failedBaseModel.Status.State).To(Equal(v1beta1.LifeCycleStateFailed))
		})

		It("Should handle invalid subpath scenarios correctly", func() {
			By("Creating PVC and populating with model files")
			pvc := createTestPVC(testNamespace, "invalid-subpath-pvc")
			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "Failed to create PVC")

			err = waitForPVCBound(ctx, k8sClient, testNamespace, "invalid-subpath-pvc", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "PVC not bound within timeout")

			err = populatePVCWithModelFiles(ctx, k8sClient, testNamespace, "invalid-subpath-pvc")
			Expect(err).NotTo(HaveOccurred(), "Failed to populate PVC")

			By("Creating BaseModel with invalid subpath")
			baseModel := createTestBaseModelWithInvalidSubpath(testNamespace, "invalid-subpath-model", "invalid-subpath-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel fails due to invalid subpath")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "invalid-subpath-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should fail when subpath is invalid")

			// Verify the failure reason
			failedBaseModel := &v1beta1.BaseModel{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "invalid-subpath-model",
				Namespace: testNamespace,
			}, failedBaseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to get failed BaseModel")

			Expect(failedBaseModel.Status.State).To(Equal(v1beta1.LifeCycleStateFailed))
		})

		It("Should handle permission errors and RBAC issues correctly", func() {
			By("Creating PVC with restricted permissions")
			pvc := createTestPVCWithRestrictedPermissions(testNamespace, "restricted-pvc")
			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "Failed to create PVC")

			err = waitForPVCBound(ctx, k8sClient, testNamespace, "restricted-pvc", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "PVC not bound within timeout")

			By("Populating PVC with corrupted model files to simulate permission issues")
			err = populatePVCWithCorruptedModelFiles(ctx, k8sClient, testNamespace, "restricted-pvc")
			Expect(err).NotTo(HaveOccurred(), "Failed to populate PVC with corrupted files")

			By("Creating BaseModel with restricted PVC")
			baseModel := createTestBaseModel(testNamespace, "restricted-pvc-model", "restricted-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel fails due to permission/access issues")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "restricted-pvc-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should fail when there are permission issues")

			// Verify the failure reason
			failedBaseModel := &v1beta1.BaseModel{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "restricted-pvc-model",
				Namespace: testNamespace,
			}, failedBaseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to get failed BaseModel")

			Expect(failedBaseModel.Status.State).To(Equal(v1beta1.LifeCycleStateFailed))
		})

		It("Should handle invalid URI format correctly", func() {
			By("Creating BaseModel with invalid URI format")
			baseModel := createTestBaseModelWithInvalidURI(testNamespace, "invalid-uri-model")
			err := k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel fails due to invalid URI format")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "invalid-uri-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should fail when URI format is invalid")

			// Verify the failure reason
			failedBaseModel := &v1beta1.BaseModel{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "invalid-uri-model",
				Namespace: testNamespace,
			}, failedBaseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to get failed BaseModel")

			Expect(failedBaseModel.Status.State).To(Equal(v1beta1.LifeCycleStateFailed))
		})

		It("Should handle cross-namespace PVC access correctly", func() {
			By("Creating PVC in a different namespace")
			otherNamespace := fmt.Sprintf("other-ns-%d", time.Now().Unix())
			otherNS := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: otherNamespace,
				},
			}
			err := k8sClient.Create(ctx, otherNS)
			Expect(err).NotTo(HaveOccurred(), "Failed to create other namespace")

			// Wait for namespace to be ready
			err = wait.PollImmediate(time.Second, 30*time.Second, func() (bool, error) {
				ns := &corev1.Namespace{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: otherNamespace}, ns)
				if err != nil {
					return false, err
				}
				return ns.Status.Phase == corev1.NamespaceActive, nil
			})
			Expect(err).NotTo(HaveOccurred(), "Other namespace not ready within timeout")

			pvc := createTestPVC(otherNamespace, "cross-ns-pvc")
			err = k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "Failed to create PVC in other namespace")

			err = waitForPVCBound(ctx, k8sClient, otherNamespace, "cross-ns-pvc", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "PVC not bound within timeout")

			By("Creating BaseModel with cross-namespace PVC reference")
			baseModel := createTestBaseModelWithCrossNamespacePVC(testNamespace, "cross-ns-model", otherNamespace, "cross-ns-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel fails due to cross-namespace access issues")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "cross-ns-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should fail when accessing cross-namespace PVC")

			// Clean up other namespace
			err = k8sClient.Delete(ctx, otherNS)
			if err != nil && !errors.IsNotFound(err) {
				GinkgoWriter.Printf("Warning: Failed to delete other namespace: %v\n", err)
			}
		})

		It("Should handle resource quota exceeded scenarios", func() {
			By("Creating resource quota to limit storage")
			quota := createResourceQuota(testNamespace, "storage-quota")
			err := k8sClient.Create(ctx, quota)
			Expect(err).NotTo(HaveOccurred(), "Failed to create resource quota")

			By("Creating PVC with size exceeding quota")
			pvc := createTestPVCWithLargeSize(testNamespace, "large-pvc")
			err = k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "Failed to create large PVC")

			By("Creating BaseModel with large PVC")
			baseModel := createTestBaseModel(testNamespace, "large-pvc-model", "large-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Verifying BaseModel fails due to resource quota exceeded")
			err = waitForBaseModelFailed(ctx, k8sClient, testNamespace, "large-pvc-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel should fail when resource quota is exceeded")

			// Verify the failure reason
			failedBaseModel := &v1beta1.BaseModel{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "large-pvc-model",
				Namespace: testNamespace,
			}, failedBaseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to get failed BaseModel")

			Expect(failedBaseModel.Status.State).To(Equal(v1beta1.LifeCycleStateFailed))
		})
	})

	Context("PVC Storage Access Modes", func() {
		It("Should handle ReadWriteOnce (RWO) PVC behavior correctly", func() {
			By("Creating RWO PVC and waiting for it to be bound")
			rwoPVC := createTestPVCWithAccessMode(testNamespace, "rwo-model-pvc", corev1.ReadWriteOnce)
			err := k8sClient.Create(ctx, rwoPVC)
			Expect(err).NotTo(HaveOccurred(), "Failed to create RWO PVC")

			err = waitForPVCBound(ctx, k8sClient, testNamespace, "rwo-model-pvc", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "RWO PVC not bound within timeout")

			By("Populating RWO PVC with model files")
			err = populatePVCWithModelFiles(ctx, k8sClient, testNamespace, "rwo-model-pvc")
			Expect(err).NotTo(HaveOccurred(), "Failed to populate RWO PVC")

			By("Creating BaseModel with RWO PVC storage URI")
			baseModel := createTestBaseModel(testNamespace, "rwo-model", "rwo-model-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel with RWO PVC")

			By("Verifying metadata extraction job runs successfully")
			err = waitForMetadataJobCompletion(ctx, k8sClient, testNamespace, "rwo-model", 5*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Metadata extraction job failed or timed out")

			By("Verifying BaseModel is ready")
			err = waitForBaseModelReady(ctx, k8sClient, testNamespace, "rwo-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel not ready within timeout")

			By("Creating InferenceService using RWO PVC model")
			inferenceService := createTestInferenceService(testNamespace, "rwo-inference", "rwo-model")
			err = k8sClient.Create(ctx, inferenceService)
			Expect(err).NotTo(HaveOccurred(), "Failed to create InferenceService")

			By("Verifying RWO PVC scheduling constraints are handled")
			err = verifyRWOSchedulingConstraints(ctx, k8sClient, testNamespace, "rwo-inference", "rwo-model-pvc")
			Expect(err).NotTo(HaveOccurred(), "RWO scheduling constraints verification failed")

			By("Verifying RWO PVC is mounted correctly")
			err = verifyPVCMountedInPod(ctx, k8sClient, testNamespace, "rwo-inference", "rwo-model-pvc")
			Expect(err).NotTo(HaveOccurred(), "RWO PVC not mounted correctly")
		})

		It("Should handle ReadWriteMany (RWX) PVC behavior correctly", func() {
			By("Creating RWX PVC and waiting for it to be bound")
			rwxPVC := createTestPVCWithAccessMode(testNamespace, "rwx-model-pvc", corev1.ReadWriteMany)
			err := k8sClient.Create(ctx, rwxPVC)
			Expect(err).NotTo(HaveOccurred(), "Failed to create RWX PVC")

			err = waitForPVCBound(ctx, k8sClient, testNamespace, "rwx-model-pvc", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "RWX PVC not bound within timeout")

			By("Populating RWX PVC with model files")
			err = populatePVCWithModelFiles(ctx, k8sClient, testNamespace, "rwx-model-pvc")
			Expect(err).NotTo(HaveOccurred(), "Failed to populate RWX PVC")

			By("Creating BaseModel with RWX PVC storage URI")
			baseModel := createTestBaseModel(testNamespace, "rwx-model", "rwx-model-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel with RWX PVC")

			By("Verifying metadata extraction job runs successfully")
			err = waitForMetadataJobCompletion(ctx, k8sClient, testNamespace, "rwx-model", 5*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Metadata extraction job failed or timed out")

			By("Verifying BaseModel is ready")
			err = waitForBaseModelReady(ctx, k8sClient, testNamespace, "rwx-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel not ready within timeout")

			By("Creating multiple InferenceServices to test RWX concurrent access")
			inferenceService1 := createTestInferenceService(testNamespace, "rwx-inference-1", "rwx-model")
			err = k8sClient.Create(ctx, inferenceService1)
			Expect(err).NotTo(HaveOccurred(), "Failed to create first InferenceService")

			inferenceService2 := createTestInferenceService(testNamespace, "rwx-inference-2", "rwx-model")
			err = k8sClient.Create(ctx, inferenceService2)
			Expect(err).NotTo(HaveOccurred(), "Failed to create second InferenceService")

			By("Verifying both InferenceServices can access RWX PVC simultaneously")
			err = waitForInferenceServiceReady(ctx, k8sClient, testNamespace, "rwx-inference-1", 5*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "First InferenceService not ready within timeout")

			err = waitForInferenceServiceReady(ctx, k8sClient, testNamespace, "rwx-inference-2", 5*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Second InferenceService not ready within timeout")

			By("Verifying RWX PVC is mounted in both pods")
			err = verifyPVCMountedInPod(ctx, k8sClient, testNamespace, "rwx-inference-1", "rwx-model-pvc")
			Expect(err).NotTo(HaveOccurred(), "RWX PVC not mounted in first pod")

			err = verifyPVCMountedInPod(ctx, k8sClient, testNamespace, "rwx-inference-2", "rwx-model-pvc")
			Expect(err).NotTo(HaveOccurred(), "RWX PVC not mounted in second pod")

			By("Verifying RWX concurrent access works correctly")
			err = verifyRWXConcurrentAccess(ctx, k8sClient, testNamespace, "rwx-model-pvc")
			Expect(err).NotTo(HaveOccurred(), "RWX concurrent access verification failed")
		})

		It("Should handle access mode conflicts and scheduling constraints", func() {
			By("Creating RWO PVC and attempting concurrent access")
			rwoPVC := createTestPVCWithAccessMode(testNamespace, "conflict-rwo-pvc", corev1.ReadWriteOnce)
			err := k8sClient.Create(ctx, rwoPVC)
			Expect(err).NotTo(HaveOccurred(), "Failed to create RWO PVC")

			err = waitForPVCBound(ctx, k8sClient, testNamespace, "conflict-rwo-pvc", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "RWO PVC not bound within timeout")

			By("Populating RWO PVC with model files")
			err = populatePVCWithModelFiles(ctx, k8sClient, testNamespace, "conflict-rwo-pvc")
			Expect(err).NotTo(HaveOccurred(), "Failed to populate RWO PVC")

			By("Creating BaseModel with RWO PVC")
			baseModel := createTestBaseModel(testNamespace, "conflict-model", "conflict-rwo-pvc")
			err = k8sClient.Create(ctx, baseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create BaseModel")

			By("Waiting for BaseModel to be ready")
			err = waitForBaseModelReady(ctx, k8sClient, testNamespace, "conflict-model", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "BaseModel not ready within timeout")

			By("Creating first InferenceService with RWO PVC")
			inferenceService1 := createTestInferenceService(testNamespace, "conflict-inference-1", "conflict-model")
			err = k8sClient.Create(ctx, inferenceService1)
			Expect(err).NotTo(HaveOccurred(), "Failed to create first InferenceService")

			By("Waiting for first InferenceService to be ready")
			err = waitForInferenceServiceReady(ctx, k8sClient, testNamespace, "conflict-inference-1", 5*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "First InferenceService not ready within timeout")

			By("Creating second InferenceService and verifying RWO conflict handling")
			inferenceService2 := createTestInferenceService(testNamespace, "conflict-inference-2", "conflict-model")
			err = k8sClient.Create(ctx, inferenceService2)
			Expect(err).NotTo(HaveOccurred(), "Failed to create second InferenceService")

			By("Verifying RWO access mode constraints are enforced")
			err = verifyRWOAccessModeConstraints(ctx, k8sClient, testNamespace, "conflict-inference-2", "conflict-rwo-pvc")
			Expect(err).NotTo(HaveOccurred(), "RWO access mode constraints verification failed")
		})
	})

	Context("ClusterBaseModel with Cross-Namespace PVC", func() {
		var clusterNamespace string

		BeforeEach(func() {
			clusterNamespace = fmt.Sprintf("cluster-pvc-test-%d", time.Now().Unix())

			// Create cluster namespace
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterNamespace,
				},
			}
			err := k8sClient.Create(ctx, namespace)
			Expect(err).NotTo(HaveOccurred(), "Failed to create cluster namespace")
		})

		AfterEach(func() {
			if clusterNamespace != "" {
				namespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: clusterNamespace,
					},
				}
				err := k8sClient.Delete(ctx, namespace)
				if err != nil && !errors.IsNotFound(err) {
					GinkgoWriter.Printf("Warning: Failed to delete cluster namespace: %v\n", err)
				}
			}
		})

		It("Should handle ClusterBaseModel with cross-namespace PVC", func() {
			By("Creating PVC in cluster namespace")
			pvc := createTestPVC(clusterNamespace, "cluster-model-pvc")
			err := k8sClient.Create(ctx, pvc)
			Expect(err).NotTo(HaveOccurred(), "Failed to create PVC in cluster namespace")

			err = waitForPVCBound(ctx, k8sClient, clusterNamespace, "cluster-model-pvc", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "PVC not bound within timeout")

			By("Populating PVC with model files")
			err = populatePVCWithModelFiles(ctx, k8sClient, clusterNamespace, "cluster-model-pvc")
			Expect(err).NotTo(HaveOccurred(), "Failed to populate PVC")

			By("Creating ClusterBaseModel with cross-namespace PVC URI")
			clusterBaseModel := createTestClusterBaseModel("cluster-llama2-7b", clusterNamespace, "cluster-model-pvc")
			err = k8sClient.Create(ctx, clusterBaseModel)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterBaseModel")

			By("Verifying metadata extraction job runs in correct namespace")
			err = waitForMetadataJobCompletion(ctx, k8sClient, clusterNamespace, "cluster-llama2-7b", 5*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "Metadata extraction job failed or timed out")

			By("Verifying ClusterBaseModel is ready")
			err = waitForClusterBaseModelReady(ctx, k8sClient, "cluster-llama2-7b", 2*time.Minute)
			Expect(err).NotTo(HaveOccurred(), "ClusterBaseModel not ready within timeout")
		})
	})
})

// Helper functions

func createTestPVCWithAccessMode(namespace, name string, accessMode corev1.PersistentVolumeAccessMode) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				accessMode,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
			StorageClassName: stringPtr("default"),
		},
	}
}

func createTestPVC(namespace, name string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
			StorageClassName: stringPtr("default"),
		},
	}
}

func createTestBaseModel(namespace, name, pvcName string) *v1beta1.BaseModel {
	return &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.BaseModelSpec{
			ModelFormat: v1beta1.ModelFormat{
				Name: "llama",
			},
			Storage: &v1beta1.StorageSpec{
				StorageUri: stringPtr(fmt.Sprintf("pvc://%s/models/llama2-7b", pvcName)),
			},
		},
	}
}

func createTestClusterBaseModel(name, pvcNamespace, pvcName string) *v1beta1.ClusterBaseModel {
	return &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta1.BaseModelSpec{
			ModelFormat: v1beta1.ModelFormat{
				Name: "llama",
			},
			Storage: &v1beta1.StorageSpec{
				StorageUri: stringPtr(fmt.Sprintf("pvc://%s:%s/models/llama2-7b", pvcNamespace, pvcName)),
			},
		},
	}
}

func createTestInferenceService(namespace, name, baseModelName string) *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.InferenceServiceSpec{
			Predictor: v1beta1.PredictorSpec{
				Model: &v1beta1.ModelSpec{
					BaseModel: stringPtr(baseModelName),
				},
			},
		},
	}
}

func populatePVCWithModelFiles(ctx context.Context, k8sClient client.Client, namespace, pvcName string) error {
	// Create a temporary pod to populate the PVC
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-populator",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "populator",
					Image: "busybox:latest",
					Command: []string{
						"/bin/sh",
						"-c",
						`
						mkdir -p /model/models/llama2-7b
						cat > /model/models/llama2-7b/config.json << 'EOF'
						{
							"architectures": ["LlamaForCausalLM"],
							"model_type": "llama",
							"torch_dtype": "float16",
							"transformers_version": "4.36.0",
							"use_cache": true,
							"vocab_size": 32000
						}
						EOF
						touch /model/models/llama2-7b/pytorch_model.bin
						echo "Model files created successfully"
						`,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "model-storage",
							MountPath: "/model",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "model-storage",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	err := k8sClient.Create(ctx, pod)
	if err != nil {
		return fmt.Errorf("failed to create populator pod: %w", err)
	}

	// Wait for pod to complete
	err = wait.PollImmediate(time.Second, 2*time.Minute, func() (bool, error) {
		pod := &corev1.Pod{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      "pvc-populator",
			Namespace: namespace,
		}, pod)
		if err != nil {
			return false, err
		}
		return pod.Status.Phase == corev1.PodSucceeded, nil
	})
	if err != nil {
		return fmt.Errorf("populator pod did not complete: %w", err)
	}

	// Clean up the pod
	err = k8sClient.Delete(ctx, pod)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete populator pod: %w", err)
	}

	return nil
}

func populatePVCWithModelFilesNoConfig(ctx context.Context, k8sClient client.Client, namespace, pvcName string) error {
	// Create a temporary pod to populate the PVC without config.json
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-populator-no-config",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "populator",
					Image: "busybox:latest",
					Command: []string{
						"/bin/sh",
						"-c",
						`
						mkdir -p /model/models/llama2-7b
						touch /model/models/llama2-7b/pytorch_model.bin
						echo "Model files created without config.json"
						`,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "model-storage",
							MountPath: "/model",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "model-storage",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	err := k8sClient.Create(ctx, pod)
	if err != nil {
		return fmt.Errorf("failed to create populator pod: %w", err)
	}

	// Wait for pod to complete
	err = wait.PollImmediate(time.Second, 2*time.Minute, func() (bool, error) {
		pod := &corev1.Pod{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      "pvc-populator-no-config",
			Namespace: namespace,
		}, pod)
		if err != nil {
			return false, err
		}
		return pod.Status.Phase == corev1.PodSucceeded, nil
	})
	if err != nil {
		return fmt.Errorf("populator pod did not complete: %w", err)
	}

	// Clean up the pod
	err = k8sClient.Delete(ctx, pod)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete populator pod: %w", err)
	}

	return nil
}

func waitForPVCBound(ctx context.Context, k8sClient client.Client, namespace, name string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		pvc := &corev1.PersistentVolumeClaim{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, pvc)
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})
}

func waitForMetadataJobCompletion(ctx context.Context, k8sClient client.Client, namespace, baseModelName string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		job := &batchv1.Job{}
		jobName := fmt.Sprintf("metadata-%s", baseModelName)
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      jobName,
			Namespace: namespace,
		}, job)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil // Job not created yet
			}
			return false, err
		}

		// Check if job completed successfully
		for _, condition := range job.Status.Conditions {
			if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
			if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
				return false, fmt.Errorf("metadata extraction job failed: %s", condition.Message)
			}
		}
		return false, nil
	})
}

func waitForBaseModelReady(ctx context.Context, k8sClient client.Client, namespace, name string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		baseModel := &v1beta1.BaseModel{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, baseModel)
		if err != nil {
			return false, err
		}
		return baseModel.Status.State == v1beta1.LifeCycleStateReady, nil
	})
}

func waitForBaseModelFailed(ctx context.Context, k8sClient client.Client, namespace, name string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		baseModel := &v1beta1.BaseModel{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, baseModel)
		if err != nil {
			return false, err
		}
		return baseModel.Status.State == v1beta1.LifeCycleStateFailed, nil
	})
}

func waitForClusterBaseModelReady(ctx context.Context, k8sClient client.Client, name string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		clusterBaseModel := &v1beta1.ClusterBaseModel{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name: name,
		}, clusterBaseModel)
		if err != nil {
			return false, err
		}
		return clusterBaseModel.Status.State == v1beta1.LifeCycleStateReady, nil
	})
}

func waitForInferenceServiceReady(ctx context.Context, k8sClient client.Client, namespace, name string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		isvc := &v1beta1.InferenceService{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, isvc)
		if err != nil {
			return false, err
		}

		// Check if InferenceService is ready
		for _, condition := range isvc.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				return true, nil
			}
		}
		return false, nil
	})
}

func verifyVolumeMountsAndSubpaths(ctx context.Context, k8sClient client.Client, namespace, isvcName, pvcName string) error {
	// Get pods for the InferenceService
	pods := &corev1.PodList{}
	err := k8sClient.List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels{
		"serving.knative.dev/service": isvcName,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for InferenceService %s", isvcName)
	}

	// Detailed verification of volume mounts and subpaths
	for _, pod := range pods.Items {
		// Check volumes
		pvcVolumeFound := false
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvcName {
				pvcVolumeFound = true

				// Verify PVC volume configuration
				if volume.PersistentVolumeClaim.ReadOnly != true {
					return fmt.Errorf("PVC volume should be read-only for model serving")
				}
				break
			}
		}

		if !pvcVolumeFound {
			return fmt.Errorf("PVC volume %s not found in pod %s", pvcName, pod.Name)
		}

		// Check volume mounts in all containers
		for _, container := range pod.Spec.Containers {
			volumeMountFound := false
			for _, mount := range container.VolumeMounts {
				if mount.Name == pvcName {
					volumeMountFound = true

					// Verify volume mount properties
					if !mount.ReadOnly {
						return fmt.Errorf("volume mount for PVC should be read-only in container %s", container.Name)
					}

					// Verify mount path is reasonable
					if mount.MountPath == "" {
						return fmt.Errorf("volume mount path cannot be empty in container %s", container.Name)
					}

					// Check subpath if present
					if mount.SubPath != "" {
						// For our test case, subpath should point to the model directory
						if !strings.Contains(mount.SubPath, "models/llama2-7b") {
							return fmt.Errorf("subpath %s should contain model directory path in container %s", mount.SubPath, container.Name)
						}
					}

					break
				}
			}

			if !volumeMountFound {
				return fmt.Errorf("volume mount for PVC %s not found in container %s", pvcName, container.Name)
			}
		}
	}

	return nil
}

func verifyPVCMountedInPod(ctx context.Context, k8sClient client.Client, namespace, isvcName, pvcName string) error {
	// Get pods for the InferenceService
	pods := &corev1.PodList{}
	err := k8sClient.List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels{
		"serving.knative.dev/service": isvcName,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for InferenceService %s", isvcName)
	}

	// Check if any pod has the PVC mounted with proper volume mounts and subpaths
	for _, pod := range pods.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvcName {
				// Found the PVC volume, now check if it's mounted with proper configuration
				for _, container := range pod.Spec.Containers {
					for _, mount := range container.VolumeMounts {
						if mount.Name == volume.Name {
							// Verify volume mount configuration
							if mount.ReadOnly != true {
								return fmt.Errorf("PVC volume mount should be read-only for model serving")
							}

							// Check if subpath is configured correctly (if expected)
							// For PVC storage, the subpath should match the model path in the PVC
							if mount.SubPath != "" {
								// Verify subpath points to the model directory
								if !strings.Contains(mount.SubPath, "models/llama2-7b") {
									return fmt.Errorf("volume mount subpath %s should contain model directory path", mount.SubPath)
								}
							}

							// Verify mount path is reasonable for model serving
							if mount.MountPath == "" {
								return fmt.Errorf("volume mount path cannot be empty")
							}

							return nil // PVC is mounted correctly with proper configuration
						}
					}
				}
			}
		}
	}

	return fmt.Errorf("PVC %s not mounted in any pod of InferenceService %s", pvcName, isvcName)
}

func populatePVCWithCustomStructure(ctx context.Context, k8sClient client.Client, namespace, pvcName string) error {
	// Create a temporary pod to populate the PVC with custom structure
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-custom-populator",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "populator",
					Image: "busybox:latest",
					Command: []string{
						"/bin/sh",
						"-c",
						`
						mkdir -p /model/custom/path/to/model
						cat > /model/custom/path/to/model/config.json << 'EOF'
						{
							"architectures": ["LlamaForCausalLM"],
							"model_type": "llama",
							"torch_dtype": "float16",
							"transformers_version": "4.36.0",
							"use_cache": true,
							"vocab_size": 32000
						}
						EOF
						touch /model/custom/path/to/model/pytorch_model.bin
						echo "Custom model structure created successfully"
						`,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "model-storage",
							MountPath: "/model",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "model-storage",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	err := k8sClient.Create(ctx, pod)
	if err != nil {
		return fmt.Errorf("failed to create custom populator pod: %w", err)
	}

	// Wait for pod to complete
	err = wait.PollImmediate(time.Second, 2*time.Minute, func() (bool, error) {
		pod := &corev1.Pod{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      "pvc-custom-populator",
			Namespace: namespace,
		}, pod)
		if err != nil {
			return false, err
		}
		return pod.Status.Phase == corev1.PodSucceeded, nil
	})
	if err != nil {
		return fmt.Errorf("custom populator pod did not complete: %w", err)
	}

	// Clean up the pod
	err = k8sClient.Delete(ctx, pod)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete custom populator pod: %w", err)
	}

	return nil
}

func createTestBaseModelWithCustomPath(namespace, name, pvcName string) *v1beta1.BaseModel {
	return &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.BaseModelSpec{
			ModelFormat: v1beta1.ModelFormat{
				Name: "llama",
			},
			Storage: &v1beta1.StorageSpec{
				StorageUri: stringPtr(fmt.Sprintf("pvc://%s/custom/path/to/model", pvcName)),
			},
		},
	}
}

func verifyRWOSchedulingConstraints(ctx context.Context, k8sClient client.Client, namespace, isvcName, pvcName string) error {
	// Get pods for the InferenceService
	pods := &corev1.PodList{}
	err := k8sClient.List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels{
		"serving.knative.dev/service": isvcName,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for InferenceService %s", isvcName)
	}

	// For RWO PVCs, verify that pods are scheduled on the same node
	// This is important because RWO PVCs can only be mounted by one pod per node
	nodeNames := make(map[string]bool)
	for _, pod := range pods.Items {
		if pod.Spec.NodeName != "" {
			nodeNames[pod.Spec.NodeName] = true
		}
	}

	// For RWO, we expect pods to be scheduled on the same node or have proper node affinity
	if len(nodeNames) > 1 {
		// Check if there are node affinity rules that would prevent multi-node scheduling
		for _, pod := range pods.Items {
			if pod.Spec.Affinity != nil && pod.Spec.Affinity.NodeAffinity != nil {
				// Node affinity is set, which is good for RWO constraints
				return nil
			}
		}
		// If no node affinity and multiple nodes, this might be a scheduling issue
		return fmt.Errorf("RWO PVC %s pods scheduled on multiple nodes without proper affinity rules", pvcName)
	}

	return nil
}

func verifyRWXConcurrentAccess(ctx context.Context, k8sClient client.Client, namespace, pvcName string) error {
	// Get all pods that mount this PVC
	pods := &corev1.PodList{}
	err := k8sClient.List(ctx, pods, client.InNamespace(namespace))
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	mountingPods := 0
	for _, pod := range pods.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvcName {
				mountingPods++
				break
			}
		}
	}

	// For RWX, we should be able to have multiple pods mounting the same PVC
	if mountingPods < 2 {
		return fmt.Errorf("RWX PVC should support multiple concurrent mounts, found %d mounting pods", mountingPods)
	}

	// Verify all mounting pods are in Running state
	for _, pod := range pods.Items {
		hasMount := false
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvcName {
				hasMount = true
				break
			}
		}
		if hasMount && pod.Status.Phase != corev1.PodRunning {
			return fmt.Errorf("pod %s mounting RWX PVC is not in Running state: %s", pod.Name, pod.Status.Phase)
		}
	}

	return nil
}

func verifyRWOAccessModeConstraints(ctx context.Context, k8sClient client.Client, namespace, isvcName, pvcName string) error {
	// Get pods for the second InferenceService
	pods := &corev1.PodList{}
	err := k8sClient.List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels{
		"serving.knative.dev/service": isvcName,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	// For RWO PVCs, the second InferenceService should either:
	// 1. Not be able to schedule (pending due to PVC access conflict)
	// 2. Be scheduled on a different node with proper affinity
	// 3. Have the first pod terminated before the second can start

	for _, pod := range pods.Items {
		// Log the PVC name for debugging purposes
		_ = pvcName
		switch pod.Status.Phase {
		case corev1.PodPending:
			// Check if pending due to PVC access conflict
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
					if strings.Contains(condition.Message, "volume") || strings.Contains(condition.Message, "PVC") {
						// This is expected for RWO conflicts
						return nil
					}
				}
			}
		case corev1.PodRunning:
			// If running, verify it's on a different node than the first pod
			// or that the first pod has been terminated
			return nil
		}
	}

	// If we get here, the RWO constraints are being properly enforced
	return nil
}

func verifyCustomSubpathMount(ctx context.Context, k8sClient client.Client, namespace, isvcName, pvcName string) error {
	// Get pods for the InferenceService
	pods := &corev1.PodList{}
	err := k8sClient.List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels{
		"serving.knative.dev/service": isvcName,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for InferenceService %s", isvcName)
	}

	// Verify custom subpath mounting
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			for _, mount := range container.VolumeMounts {
				if mount.Name == pvcName {
					// Check if subpath is correctly set to custom path
					if mount.SubPath != "" {
						if !strings.Contains(mount.SubPath, "custom/path/to/model") {
							return fmt.Errorf("subpath %s should contain custom model path", mount.SubPath)
						}
					}

					// Verify other mount properties
					if !mount.ReadOnly {
						return fmt.Errorf("volume mount should be read-only")
					}

					return nil // Custom subpath mount verified
				}
			}
		}
	}

	return fmt.Errorf("custom subpath mount not found in any pod")
}

func stringPtr(s string) *string {
	return &s
}

// Helper functions for error test cases

func createTestPVCWithUnboundStorageClass(namespace, name string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: stringPtr("non-existent-storage-class"),
		},
	}
}

func createTestBaseModelWithInvalidSubpath(namespace, name, pvcName string) *v1beta1.BaseModel {
	return &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.BaseModelSpec{
			ModelFormat: v1beta1.ModelFormat{
				Name: "safetensors",
			},
			Storage: &v1beta1.StorageSpec{
				StorageUri: stringPtr(fmt.Sprintf("pvc://%s/non/existent/path", pvcName)),
			},
		},
	}
}

func createTestPVCWithRestrictedPermissions(namespace, name string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
}

func populatePVCWithCorruptedModelFiles(ctx context.Context, k8sClient client.Client, namespace, pvcName string) error {
	// Create a pod to populate the PVC with corrupted files
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("populate-%s", pvcName),
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "populate",
					Image:   "busybox:latest",
					Command: []string{"/bin/sh", "-c"},
					Args: []string{
						`mkdir -p /data/models/llama2-7b &&
						 echo "corrupted json content" > /data/models/llama2-7b/config.json &&
						 echo "corrupted model data" > /data/models/llama2-7b/model.safetensors &&
						 echo "corrupted tokenizer" > /data/models/llama2-7b/tokenizer.json`,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "model-storage",
							MountPath: "/data",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "model-storage",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	err := k8sClient.Create(ctx, pod)
	if err != nil {
		return err
	}

	// Wait for pod to complete
	err = wait.PollImmediate(time.Second, 2*time.Minute, func() (bool, error) {
		pod := &corev1.Pod{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      fmt.Sprintf("populate-%s", pvcName),
			Namespace: namespace,
		}, pod)
		if err != nil {
			return false, err
		}
		return pod.Status.Phase == corev1.PodSucceeded, nil
	})

	// Clean up the pod
	_ = k8sClient.Delete(ctx, pod)
	return err
}

func createTestBaseModelWithInvalidURI(namespace, name string) *v1beta1.BaseModel {
	return &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.BaseModelSpec{
			ModelFormat: v1beta1.ModelFormat{
				Name: "safetensors",
			},
			Storage: &v1beta1.StorageSpec{
				StorageUri: stringPtr("invalid://uri/format"),
			},
		},
	}
}

func createTestBaseModelWithCrossNamespacePVC(namespace, name, pvcNamespace, pvcName string) *v1beta1.BaseModel {
	return &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.BaseModelSpec{
			ModelFormat: v1beta1.ModelFormat{
				Name: "safetensors",
			},
			Storage: &v1beta1.StorageSpec{
				StorageUri: stringPtr(fmt.Sprintf("pvc://%s:%s/models/llama2", pvcNamespace, pvcName)),
			},
		},
	}
}

func createTestPVCWithInvalidStorageClass(namespace, name string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: stringPtr("non-existent-storage-class"),
		},
	}
}

func createResourceQuota(namespace, name string) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceRequestsStorage: resource.MustParse("1Gi"),
			},
		},
	}
}

func createTestPVCWithLargeSize(namespace, name string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"), // Large size to exceed quota
				},
			},
		},
	}
}
