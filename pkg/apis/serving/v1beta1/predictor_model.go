package v1beta1

import (
	"context"
	goerrors "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	"sort"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ModelSpec struct {
	// Specific ClusterServingRuntime/ServingRuntime name to use for deployment.
	// +optional
	Runtime *string `json:"runtime,omitempty"`

	PredictorExtensionSpec `json:",inline"`

	// +required Specific ClusterBaseModel/BaseModel name to use for hosting the model.
	BaseModel *string `json:"baseModel,omitempty"`

	// +optional Specific FineTunedWeight name to use for hosting the additional weights.
	FineTunedWeights []string `json:"fineTunedWeights,omitempty"`
}

var (
	_ ComponentImplementation = &ModelSpec{}
)

// Here, the ComponentImplementation interface is implemented in order to maintain the
// component validation logic. This will probably be refactored out eventually.

func (m *ModelSpec) Default(config *InferenceServicesConfig) {}

func (m *ModelSpec) GetContainer(metadata metav1.ObjectMeta, extensions *ComponentExtensionSpec, config *InferenceServicesConfig, predictorHost ...string) *v1.Container {
	return &m.Container
}

func (m *ModelSpec) GetProtocol() constants.InferenceServiceProtocol {
	if m.ProtocolVersion != nil {
		return *m.ProtocolVersion
	}
	return constants.OpenInferenceProtocolV2
}

type stringSet map[string]struct{}

func (ss stringSet) add(s string) {
	ss[s] = struct{}{}
}

func (ss stringSet) contains(s string) bool {
	_, found := ss[s]
	return found
}

// GetBaseModel Get the base model name from the given model name.
func GetBaseModel(cl client.Client, name string, namespace string) (*BaseModelSpec, error) {
	baseModel := &BaseModel{}
	err := cl.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, baseModel)
	if err == nil {
		return &baseModel.Spec, nil
	} else if !errors.IsNotFound(err) {
		return nil, err
	}
	clusterBaseModel := &ClusterBaseModel{}
	err = cl.Get(context.TODO(), client.ObjectKey{Name: name}, clusterBaseModel)
	if err == nil {
		return &clusterBaseModel.Spec, nil
	} else if !errors.IsNotFound(err) {
		return nil, err
	}
	return nil, goerrors.New("No BaseModel or ClusterBaseModel with the name: " + name)
}

// GetSupportingRuntimes Get a list of ServingRuntimeSpecs that correspond to ServingRuntimes and ClusterServingRuntimes that
// support the given model.
func (m *ModelSpec) GetSupportingRuntimes(cl client.Client, namespace string) ([]SupportedRuntime, error) {
	modelProtocolVersion := m.GetProtocol()

	// List all namespace-scoped runtimes.
	runtimes := &ServingRuntimeList{}
	if err := cl.List(context.TODO(), runtimes, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	// Sort namespace-scoped runtimes by created timestamp desc and name asc.
	sortServingRuntimeList(runtimes)

	// List all cluster-scoped runtimes.
	clusterRuntimes := &ClusterServingRuntimeList{}
	if err := cl.List(context.TODO(), clusterRuntimes); err != nil {
		return nil, err
	}
	// Sort cluster-scoped runtimes by created timestamp desc and name asc.
	sortClusterServingRuntimeList(clusterRuntimes)

	var srSpecs []SupportedRuntime
	var clusterSrSpecs []SupportedRuntime

	model, err := GetBaseModel(cl, namespace, *m.BaseModel)
	if err != nil {
		return nil, err
	}

	for i := range runtimes.Items {
		rt := &runtimes.Items[i]
		if !rt.Spec.IsDisabled() && m.RuntimeSupportsModel(&rt.Spec, model) && rt.Spec.IsProtocolVersionSupported(modelProtocolVersion) {
			srSpecs = append(srSpecs, SupportedRuntime{Name: rt.GetName(), Spec: rt.Spec})
		}
	}
	sortSupportedRuntimeByPriority(srSpecs, model.ModelFormat)
	for i := range clusterRuntimes.Items {
		crt := &clusterRuntimes.Items[i]
		if !crt.Spec.IsDisabled() && m.RuntimeSupportsModel(&crt.Spec, model) && crt.Spec.IsProtocolVersionSupported(modelProtocolVersion) {
			clusterSrSpecs = append(clusterSrSpecs, SupportedRuntime{Name: crt.GetName(), Spec: crt.Spec})
		}
	}
	sortSupportedRuntimeByPriority(clusterSrSpecs, model.ModelFormat)
	srSpecs = append(srSpecs, clusterSrSpecs...)
	return srSpecs, nil
}

// RuntimeSupportsModel Check if the given runtime supports the specified model.
func (m *ModelSpec) RuntimeSupportsModel(srSpec *ServingRuntimeSpec, modelSpec *BaseModelSpec) bool {
	// assignment to a runtime depends on the model format labels
	runtimeLabelSet := m.getServingRuntimeSupportedModelFormatLabelSet(srSpec.SupportedModelFormats)
	modelLabel := m.getModelFormatLabel(modelSpec)
	// if the runtime has the model's label, then it supports that model.
	return runtimeLabelSet.contains(modelLabel)
}

func (m *ModelSpec) getModelFormatLabel(modelSpec *BaseModelSpec) string {
	mt := modelSpec.ModelFormat
	label := "mt:" + mt.Name

	if mt.Version != nil {
		label += ":" + *mt.Version
	}
	if modelSpec.ModelArchitecture != nil {
		label += ":" + *modelSpec.ModelArchitecture
	}
	if modelSpec.ModelType != nil {
		label += ":" + *modelSpec.ModelType
	}

	return label
}

func (m *ModelSpec) getServingRuntimeSupportedModelFormatLabelSet(supportedModelFormats []SupportedModelFormat) stringSet {
	set := make(stringSet, 2*len(supportedModelFormats)+1)

	// model format labels
	for _, t := range supportedModelFormats {
		// If runtime isn't explicitly set, only add labels for modelFormats where AutoSelect is true.
		if m.Runtime != nil || (t.AutoSelect != nil && *t.AutoSelect) {
			label := "mt:" + t.Name
			if t.Version != nil {
				label += ":" + *t.Version
			}
			if t.ModelArchitecture != nil {
				label += ":" + *t.ModelArchitecture
			}
			if t.ModelType != nil {
				label += ":" + *t.ModelType
			}
			set.add(label)
		}
	}
	return set
}

func sortServingRuntimeList(runtimes *ServingRuntimeList) {
	sort.Slice(runtimes.Items, func(i, j int) bool {
		if runtimes.Items[i].CreationTimestamp.Before(&runtimes.Items[j].CreationTimestamp) {
			return false
		}
		if runtimes.Items[j].CreationTimestamp.Before(&runtimes.Items[i].CreationTimestamp) {
			return true
		}
		return runtimes.Items[i].Name < runtimes.Items[j].Name
	})
}

func sortClusterServingRuntimeList(runtimes *ClusterServingRuntimeList) {
	sort.Slice(runtimes.Items, func(i, j int) bool {
		if runtimes.Items[i].CreationTimestamp.Before(&runtimes.Items[j].CreationTimestamp) {
			return false
		}
		if runtimes.Items[j].CreationTimestamp.Before(&runtimes.Items[i].CreationTimestamp) {
			return true
		}
		return runtimes.Items[i].Name < runtimes.Items[j].Name
	})
}

func sortSupportedRuntimeByPriority(runtimes []SupportedRuntime, modelFormat ModelFormat) {
	sort.Slice(runtimes, func(i, j int) bool {
		p1 := runtimes[i].Spec.GetPriority(modelFormat.Name)
		p2 := runtimes[j].Spec.GetPriority(modelFormat.Name)

		switch {
		case p1 == nil && p2 == nil: // if both runtimes does not specify the priority, the order is kept.
			return false
		case p1 == nil && p2 != nil: // runtime with priority specified takes precedence
			return false
		case p1 != nil && p2 == nil:
			return true
		}
		return *p1 > *p2
	})
}
