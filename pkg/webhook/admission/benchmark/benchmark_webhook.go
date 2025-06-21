package benchmark

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/sgl-project/ome/pkg/constants"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1beta1 "github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	storageutil "github.com/sgl-project/ome/pkg/utils/storage"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var log = logf.Log.WithName(constants.BenchmarkJobValidatorWebhookName)

// BenchmarkJobValidator validates BenchmarkJob objects.
type BenchmarkJobValidator struct {
	Client  client.Client
	Decoder admission.Decoder
}

// +kubebuilder:webhook:path=/validate-ome-io-benchmark-job,mutating=false,failurePolicy=fail,groups=serving.ome.io,resources=benchmarkjobs,,verbs=create;update,versions=v1beta1,name=benchmarkjob.ome-webhook-server.validator,sideEffects=None,admissionReviewVersions=v1

// Handle implements webhook.Validator so a webhook will be registered for the type
func (v *BenchmarkJobValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	benchmarkJob := &v1beta1.BenchmarkJob{}

	if err := v.Decoder.Decode(req, benchmarkJob); err != nil {
		log.Error(err, "Failed to decode benchmark job", "name", benchmarkJob.Name, "namespace", benchmarkJob.Namespace)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := v.validateBenchmarkJob(ctx, benchmarkJob); err != nil {
		log.Error(err, "Validation failed for BenchmarkJob", "namespace", benchmarkJob.Namespace, "name", benchmarkJob.Name)
		return admission.Denied(err.Error())
	}

	return admission.Allowed("Validation passed")
}

func (v *BenchmarkJobValidator) validateBenchmarkJob(ctx context.Context, benchmarkJob *v1beta1.BenchmarkJob) error {

	// Validate endpoint
	if err := v.validateEndpoint(benchmarkJob.Spec.Endpoint); err != nil {
		return fmt.Errorf("invalid endpoint: %w", err)
	}

	// Validate Traffic Scenarios
	if err := v.validateTrafficScenarios(benchmarkJob.Spec.Task, benchmarkJob.Spec.TrafficScenarios); err != nil {
		return fmt.Errorf("invalid traffic scenarios: %w", err)
	}

	// Validate Additional Request Parameters
	if err := v.validateAdditionalRequestParams(benchmarkJob.Spec.AdditionalRequestParams); err != nil {
		return fmt.Errorf("invalid additional request parameters: %w", err)
	}

	// Validate Storage
	if err := v.validateStorage(benchmarkJob.Spec.OutputLocation); err != nil {
		return fmt.Errorf("invalid storage: %w", err)
	}

	return nil
}

func (v *BenchmarkJobValidator) validateEndpoint(endpoint v1beta1.EndpointSpec) error {
	if endpoint.Endpoint == nil && endpoint.InferenceService == nil {
		return fmt.Errorf("endpoint or InferenceService must be specified")
	}
	if endpoint.Endpoint != nil && endpoint.InferenceService != nil {
		return fmt.Errorf("endpoint and InferenceService cannot be specified together")
	}
	return nil
}

func (v *BenchmarkJobValidator) validateTrafficScenarios(task string, scenarios []string) error {
	// Define default scenarios for each task
	defaultScenarios := map[string][]string{
		"text-to-text": {
			"N(480,240)/(300,150)",
			"D(100,100)",
			"D(100,1000)",
			"D(2000,200)",
			"D(7800,200)"},
		"image-to-text": {
			"I(512,512)",
			"I(1024,512)",
			"I(2048,2048)"},
		"text-to-embeddings": {
			"E(64)",
			"E(128)",
			"E(256)",
			"E(512)",
			"E(1024)"},
		"image-to-embeddings": {"I(512,512)"},
	}

	// Use default if no scenarios provided
	if len(scenarios) == 0 {
		scenarios = defaultScenarios[task]
	}

	// Validate each scenario
	for _, scenario := range scenarios {
		if err := validateScenario(task, scenario); err != nil {
			return fmt.Errorf("failed to validate scenario '%s': %w", scenario, err)
		}
	}
	return nil
}

func (v *BenchmarkJobValidator) validateStorage(storage *v1beta1.StorageSpec) error {
	if storage == nil {
		return nil
	}

	if storage.StorageUri == nil {
		return fmt.Errorf("storageUri cannot be empty")
	}

	err := storageutil.ValidateStorageURI(*storage.StorageUri)
	if err != nil {
		return fmt.Errorf("error parsing storage URI: %v", err)
	}

	return nil
}

// ScenarioValidationPattern holds regex patterns for scenario validation.
var ScenarioValidationPattern = map[string]*regexp.Regexp{
	"text-to-text":        regexp.MustCompile(`^N\(\d+,\d+\)\/\(\d+,\d+\)|U\(\d+,\d+\)(?:\/\(\d+,\d+\))?|D\(\d+,\d+\)$`),
	"text-to-embeddings":  regexp.MustCompile(`^E\(\d+,\d+\)$`),
	"image-to-text":       regexp.MustCompile(`^I\(\d+,\d+(?:,\d+)?\)$`),
	"image-to-embeddings": regexp.MustCompile(`^I\(\d+,\d+(?:,\d+)?\)$`),
}

// validateScenario validates a scenario string based on its task.
func validateScenario(task string, scenario string) error {
	pattern, exists := ScenarioValidationPattern[task]
	if !exists {
		return fmt.Errorf("no validation pattern defined for task: %s", task)
	}

	if !pattern.MatchString(scenario) {
		return fmt.Errorf("invalid scenario format for task '%s': %s", task, scenario)
	}

	return nil
}

func (v *BenchmarkJobValidator) validateAdditionalRequestParams(params map[string]string) error {
	for key, value := range params {
		switch key {
		case "temperature":
			temperature, err := parseFloat(value)
			if err != nil {
				log.Error(err, "Failed to parse temperature", "temperature", value)
				return fmt.Errorf("invalid temperature: %w", err)
			}
			if temperature > 1.5 {
				log.Info("Warning: temperature is too high", "temperature", temperature)
			}
		case "ignore_eos":
			if value != "true" && value != "false" {
				return fmt.Errorf("ignore_eos must be 'true' or 'false'")
			}
		}
	}
	return nil
}

func parseFloat(value string) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
