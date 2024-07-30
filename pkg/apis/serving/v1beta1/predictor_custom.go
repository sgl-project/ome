package v1beta1

import (
	"fmt"
	"strings"

	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/gen/ome/pkg/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CustomPredictor defines arguments for configuring a custom server.
type CustomPredictor struct {
	v1.PodSpec `json:",inline"`
}

var (
	_ ComponentImplementation = &CustomPredictor{}
	_ PredictorImplementation = &CustomPredictor{}
)

func NewCustomPredictor(podSpec *PodSpec) *CustomPredictor {
	return &CustomPredictor{PodSpec: v1.PodSpec(*podSpec)}
}

// Validate returns an error if invalid
func (c *CustomPredictor) Validate() error {
	return utils.FirstNonNilError([]error{
		c.validateCustomProtocol(),
	})
}

func (c *CustomPredictor) validateCustomProtocol() error {
	for _, envVar := range c.Containers[0].Env {
		if envVar.Name == constants.CustomSpecProtocolEnvVarKey {
			if envVar.Value == string(constants.OpenInferenceProtocolV2) || envVar.Value == string(constants.OpenInferenceProtocolV1) {
				return nil
			} else {
				return fmt.Errorf(InvalidProtocol, strings.Join([]string{string(constants.OpenInferenceProtocolV1), string(constants.OpenInferenceProtocolV2)}, ", "), envVar.Value)
			}
		}
	}
	return nil
}

// Default sets defaults on the resource
func (c *CustomPredictor) Default(config *InferenceServicesConfig) {
	if len(c.Containers) == 0 {
		c.Containers = append(c.Containers, v1.Container{})
	}
	c.Containers[0].Name = constants.InferenceServiceContainerName
	setResourceRequirementDefaults(&c.Containers[0].Resources)
}

func (c *CustomPredictor) GetStorageUri() *string {
	// return the CustomSpecStorageUri env variable value if set on the spec
	for _, container := range c.Containers {
		if container.Name == constants.InferenceServiceContainerName {
			for _, envVar := range container.Env {
				if envVar.Name == constants.CustomSpecStorageUriEnvVarKey {
					return &envVar.Value
				}
			}
			break
		}
	}
	return nil
}

func (c *CustomPredictor) GetStorageSpec() *StorageSpec {
	return nil
}

// GetContainer transforms the resource into a container spec
func (c *CustomPredictor) GetContainer(metadata metav1.ObjectMeta, extensions *ComponentExtensionSpec, config *InferenceServicesConfig,
	predictorHost ...string) *v1.Container {
	for _, container := range c.Containers {
		if container.Name == constants.InferenceServiceContainerName {
			return &container
		}
	}
	return nil
}

func (c *CustomPredictor) GetProtocol() constants.InferenceServiceProtocol {
	// Handle collocation of transformer and predictor scenario
	for _, container := range c.Containers {
		if container.Name == constants.TransformerContainerName {
			for _, envVar := range container.Env {
				if envVar.Name == constants.CustomSpecProtocolEnvVarKey {
					return constants.InferenceServiceProtocol(envVar.Value)
				}
			}
			return constants.OpenAIProtocol
		}
	}
	for _, envVar := range c.Containers[0].Env {
		if envVar.Name == constants.CustomSpecProtocolEnvVarKey {
			return constants.InferenceServiceProtocol(envVar.Value)
		}
	}
	return constants.OpenAIProtocol
}
