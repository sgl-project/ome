package namespace

import (
	"context"
	"reflect"
	"strconv"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("NamespaceReconciler")

type NamespaceReconciler struct {
	client    client.Client
	scheme    *runtime.Scheme
	Namespace *v1.Namespace
}

func NewNamespaceReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	namespaceName string,
	enableKueue bool,
) (*NamespaceReconciler, error) {
	namespace := createNamespace(namespaceName, enableKueue)
	return &NamespaceReconciler{
		client:    client,
		scheme:    scheme,
		Namespace: namespace,
	}, nil
}

func createNamespace(namespaceName string, enableKueue bool) *v1.Namespace {
	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
			Labels: map[string]string{
				constants.KueueWorkloadNamespaceSelectorLabelKey: strconv.FormatBool(enableKueue),
			},
		},
	}
	return namespace
}

func (r *NamespaceReconciler) checkNamespaceExist(client client.Client) (constants.CheckResultType, *v1.Namespace, error) {
	//get namespace
	existingNamespace := &v1.Namespace{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: r.Namespace.ObjectMeta.Name}, existingNamespace)
	if err != nil {
		if apierr.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		return constants.CheckResultUnknown, nil, err
	}
	if namespaceEquals(r.Namespace, existingNamespace) {
		return constants.CheckResultExisted, existingNamespace, nil
	}
	return constants.CheckResultUpdate, existingNamespace, nil
}

// TODO: add more logic later and delete annotation compare
// during ome migration, the namespace will contain the annotation kubectl.kubernetes.io/last-applied-configuration and desired namespace doesn't have this annotation
// we can use this annotation to check if current namespace need to update.
// we need to let namespace reconciler to update the namespace owned by v1alpha1 DAC.
func namespaceEquals(desired, existing *v1.Namespace) bool {
	// check labels. current desired namespace doesn't have labels now, everytime checking will get false
	//if !areMapsEqual(desired.ObjectMeta.Labels, existing.ObjectMeta.Labels) {
	//	log.Info("namespace labels differ",
	//		"namespace", desired.Name,
	//		"desired", desired.ObjectMeta.Labels,
	//		"existing", existing.ObjectMeta.Labels)
	//	//return false
	//}

	// Compare annotations - may contain important configuration metadata
	// current desired namespace doesn't have annotations now.
	// existing v1beta1 namespace doesn't have annotations either.
	if !reflect.DeepEqual(desired.ObjectMeta.Annotations, existing.ObjectMeta.Annotations) {
		log.Info("namespace annotations differ",
			"namespace", desired.Name,
			"desired", desired.ObjectMeta.Annotations,
			"existing", existing.ObjectMeta.Annotations)
		return false
	}
	return true
}

func (r *NamespaceReconciler) Reconcile() (*v1.Namespace, error) {
	// reconcile namespace
	checkResult, existingNamespace, err := r.checkNamespaceExist(r.client)
	log.Info("namespace reconcile", "checkResult", checkResult, "err", err, "existingNamespace", existingNamespace)
	if err != nil {
		return nil, err
	}
	if checkResult == constants.CheckResultCreate {
		err = r.client.Create(context.TODO(), r.Namespace)
		if err != nil {
			return nil, err
		} else {
			return r.Namespace, nil
		}
	} else if checkResult == constants.CheckResultUpdate { // check namespace update
		err = r.client.Update(context.TODO(), r.Namespace)
		if err != nil {
			return nil, err
		} else {
			return r.Namespace, nil
		}
	} else {
		return existingNamespace, nil
	}

}
