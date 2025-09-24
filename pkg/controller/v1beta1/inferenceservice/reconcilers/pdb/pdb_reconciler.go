package pdb

import (
	"context"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/constants"

	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("PDBReconciler")

// PDBReconciler is the struct of Raw K8S Object
type PDBReconciler struct {
	client       client.Client
	scheme       *runtime.Scheme
	PDB          *policyv1.PodDisruptionBudget
	componentExt *v1beta1.ComponentExtensionSpec
}

func NewPDBReconciler(client client.Client,
	scheme *runtime.Scheme,
	componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec) *PDBReconciler {
	return &PDBReconciler{
		client:       client,
		scheme:       scheme,
		PDB:          createPDB(componentMeta, componentExt),
		componentExt: componentExt,
	}
}

func createPDB(componentMeta metav1.ObjectMeta,
	componentExt *v1beta1.ComponentExtensionSpec) *policyv1.PodDisruptionBudget {

	var minAvailable *intstr.IntOrString
	var maxUnavailable *intstr.IntOrString

	if componentExt.MinAvailable != nil {
		minAvailable = componentExt.MinAvailable
	}

	if componentExt.MaxUnavailable != nil {
		maxUnavailable = componentExt.MaxUnavailable
	}

	if componentExt.MinAvailable == nil && componentExt.MaxUnavailable == nil {
		// Set maxUnavailable = 1 as default
		maxUnavailable = &intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: 1,
		}
	}

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: componentMeta,
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable:   minAvailable,
			MaxUnavailable: maxUnavailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": constants.GetRawServiceLabel(componentMeta.Name),
				},
			},
		},
	}
	return pdb
}

// checkPDBExist checks if the pdb exists
func (r *PDBReconciler) checkPDBExist(client client.Client) (constants.CheckResultType, *policyv1.PodDisruptionBudget, error) {
	//get pdb
	existingPDB := &policyv1.PodDisruptionBudget{}
	err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: r.PDB.ObjectMeta.Namespace,
		Name:      r.PDB.ObjectMeta.Name,
	}, existingPDB)
	if err != nil {
		if apierr.IsNotFound(err) {
			return constants.CheckResultCreate, nil, nil
		}
		log.Info("Failed to get existing PDB", "Namespace", r.PDB.ObjectMeta.Namespace, "Name", r.PDB.ObjectMeta.Name)
		return constants.CheckResultUnknown, nil, err
	}

	//existed, check equivalent
	if semanticPDBEquals(r.PDB, existingPDB) {
		return constants.CheckResultExisted, existingPDB, nil
	}
	return constants.CheckResultUpdate, existingPDB, nil
}

func semanticPDBEquals(desired, existing *policyv1.PodDisruptionBudget) bool {
	if desired.Spec.MaxUnavailable != nil && existing.Spec.MaxUnavailable != nil {
		if !equality.Semantic.DeepEqual(*desired.Spec.MaxUnavailable, *existing.Spec.MaxUnavailable) {
			return false
		}
	} else if desired.Spec.MaxUnavailable != nil || existing.Spec.MaxUnavailable != nil {
		return false
	}

	if desired.Spec.MinAvailable != nil && existing.Spec.MinAvailable != nil {
		if !equality.Semantic.DeepEqual(*desired.Spec.MinAvailable, *existing.Spec.MinAvailable) {
			return false
		}
	} else if desired.Spec.MinAvailable != nil || existing.Spec.MinAvailable != nil {
		return false
	}

	return equality.Semantic.DeepEqual(*desired.Spec.Selector, *existing.Spec.Selector)
}

// Reconcile ...
func (r *PDBReconciler) Reconcile() (*policyv1.PodDisruptionBudget, error) {
	checkResult, existingPDB, err := r.checkPDBExist(r.client)
	log.Info("service reconcile", "checkResult", checkResult, "err", err)
	if err != nil {
		return nil, err
	}

	switch checkResult {
	case constants.CheckResultCreate:
		err = r.client.Create(context.TODO(), r.PDB)
		if err != nil {
			log.Error(err, "Failed to create PDB", "Namespace", r.PDB.ObjectMeta.Namespace, "Name", r.PDB.ObjectMeta.Name)
			return nil, err
		}
		return r.PDB, nil
	case constants.CheckResultUpdate:
		// Preserve resourceVersion to avoid 409 Conflict on update
		if existingPDB != nil {
			r.PDB.SetResourceVersion(existingPDB.GetResourceVersion())
		}
		err = r.client.Update(context.TODO(), r.PDB)
		if err != nil {
			log.Info("Failed to update PDB", "Namespace", r.PDB.ObjectMeta.Namespace, "Name", r.PDB.ObjectMeta.Name)
			return nil, err
		}
		return r.PDB, nil
	default:
		return existingPDB, nil
	}
}
