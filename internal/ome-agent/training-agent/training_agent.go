package training_agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sgl-project/sgl-ome/pkg/casper"
	"github.com/sgl-project/sgl-ome/pkg/constants"
	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/sgl-project/sgl-ome/pkg/zipper"
)

// constants for Multipart Upload
const (
	BigFileSizeInMB              = 200
	DefaultDownloadChunkSizeInMB = 20
	DefaultUploadChunkSizeInMB   = 50
	DefaultDownloadThreads       = 10
	DefaultUploadThreads         = 10
)

// TrainingAgent represents a training sidecar application
type TrainingAgent struct {
	logger   logging.Interface
	Config   Config
	FTClient FTClient
}

// NewTrainingAgent constructs a new training agent from the given configuration.
func NewTrainingAgent(config *Config) (*TrainingAgent, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("training agent config invalid: %v", err)
	}
	ftClient, err := NewFTClient(config)
	if err != nil {
		return nil, err
	}

	return &TrainingAgent{
		logger:   config.AnotherLogger,
		Config:   *config,
		FTClient: ftClient,
	}, nil
}

// Start starts the application
func (d *TrainingAgent) Start() {
	d.logger.Infof("Starting %s Training Agent", d.Config.Runtime)

	d.downloadData()

	d.startTraining()

	d.monitoringTraining()

	d.zipTrainedModel()

	d.uploadModelToObjectStorage()

	d.uploadTrainingMetricsToObjectStorage()

	d.terminateTrainingInstance()
}

func (d *TrainingAgent) downloadData() {
	d.logger.Infof("Start downloading data")
	d.logger.Infof("Training data store url: %+v", *d.Config.TrainingDataObjectStoreURI)
	d.logger.Infof("Training data local store path: %+v", d.Config.TrainingDataStoreDirectory)

	if err := d.Config.InputObjectStorageDataStore.DownloadBasedOnObjectSize(
		*d.Config.TrainingDataObjectStoreURI,
		d.Config.TrainingDataStoreDirectory,
		true,
		BigFileSizeInMB,
		DefaultDownloadChunkSizeInMB,
		DefaultDownloadThreads); err != nil {

		panic(fmt.Errorf("failed to download training data: %+v", err))
	}

	d.logger.Info("Done with downloading training data")
}

func (d *TrainingAgent) startTraining() {
	d.logger.Infof("Kicking off training on endpoint: %s", constants.TrainingEndpoint)

	startTime := time.Now()
	jsonPayload := d.prepareFTDetailsPayload()
	var response *http.Response
	var err error
	for {
		response, err = d.FTClient.PostFineTune(jsonPayload)
		if err != nil {
			elapsed := time.Since(startTime)
			if elapsed > constants.Timeout {
				d.logger.Errorf("can't start finetune server in %d mins", constants.Timeout)
				panic(fmt.Errorf("can't start finetune server: %+v", err))
			}
			d.logger.Infof("sever is starting, checking again in 1 minute...")
			time.Sleep(constants.RetryInterval)
			continue
		}
		break
	}

	responseBody, err := readResponseBody(response)
	if err != nil {
		d.logger.Warnf("failed to read response body: %+v", err)
	}
	if response.StatusCode == http.StatusOK || response.StatusCode == http.StatusAccepted {
		d.logger.Infof("/finetune - StatusCode: %d, Response: %s", response.StatusCode, string(responseBody))
		return
	} else {
		d.logger.Errorf("/finetune - StatusCode: %d, Response: %s", response.StatusCode, string(responseBody))

		/*
		 * For llama3 peft, data validation processed synchronously in its /finetune API.
		 * (while for cohere ft, data validation processed by an async way and all these preprocessing errors being got via /status)
		 */
		if d.Config.Runtime == constants.PeftTrainingSidecar {
			d.handlePeftDataError(response, responseBody)
		}
		panic(fmt.Errorf("error calling /finetune: %s", string(responseBody)))
	}
}

func (d *TrainingAgent) monitoringTraining() {
Loop:
	for {
		response, err := d.FTClient.GetStatus()
		if err != nil {
			panic(fmt.Errorf("failed to call /status: %+v", err))
		}
		// read response body
		body, err := readResponseBody(response)
		if err != nil {
			panic(fmt.Errorf("failed to read /status response: %+v", err))
		}
		if response.StatusCode != http.StatusOK {
			panic(fmt.Errorf("error calling /status - StatusCode %d, Response: %s", response.StatusCode, string(body)))
		}

		// unmarshall response body
		var unmarshalledResp Response
		err = json.Unmarshal(body, &unmarshalledResp)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal /status response %s: %+v", string(body), err))
		}

		d.logger.Infof("/status - StatusCode: %d, Response: %s", response.StatusCode, unmarshalledResp.Message)

		switch d.Config.Runtime {
		case constants.CohereCommand1TrainingSidecar, constants.CohereCommandRTrainingSidecar:
			finished, err := d.handleCohereTrainingStatus(unmarshalledResp)
			if err != nil {
				_ = d.handleDataError(unmarshalledResp.Message) // side error, not the main one from training container which failed training, not throw it out for a better main error determination via logs
				d.logger.Info("Terminating training process..")
				d.terminateTrainingInstance()
				panic(err)
			}

			if finished {
				break Loop
			}
		case constants.PeftTrainingSidecar:
			finished, err := d.handlePeftTrainingStatus(unmarshalledResp)
			if err != nil {
				// For peft, no need to handle data error here in /status response (handle it in /finetune response)
				d.logger.Info("Terminating training process..")
				d.terminateTrainingInstance()
				panic(err)
			}

			if finished {
				break Loop
			}
		default:
			panic(fmt.Errorf("unknown runtime %s", d.Config.Runtime))
		}
		time.Sleep(1 * time.Minute)
	}
}

func (d *TrainingAgent) zipTrainedModel() {
	switch d.Config.Runtime {
	case constants.CohereCommand1TrainingSidecar:
		d.zipCohereFTWeights() // for cohere fastertransformer FT (tfew & vanilla)
	case constants.PeftTrainingSidecar:
		d.zipFTWeightsAndMergedWeights(constants.PeftFineTunedWeightsDirectory, constants.PeftMergedWeightsDirectory)
	case constants.CohereCommandRTrainingSidecar:
		if d.Config.CohereFineTuneDetails.Strategy == constants.TFewTrainingStrategy {
			d.zipFTWeightsAndMergedWeights(constants.CohereCommandRTFewFTWeightsDirectory, constants.CohereCommandRMergedWeightsDirectory)
		} else if d.Config.CohereFineTuneDetails.Strategy == constants.LoraTrainingStrategy {
			if d.Config.CohereFineTuneDetails.ServingStrategy == constants.VanillaServingStrategy {
				d.zipFTWeightsAndMergedWeights(constants.CohereCommandRLoraFineTunedWeightsDirectory, constants.CohereCommandRMergedWeightsDirectory)
			} else if d.Config.CohereFineTuneDetails.ServingStrategy == constants.LoraServingStrategy {
				d.zipCohereFTWeights()
			}
		}
	default:
		panic(fmt.Errorf("unknown runtime %s", d.Config.Runtime))
	}
}

func (d *TrainingAgent) uploadModelToObjectStorage() {
	// upload ft model weights to bucket
	d.logger.Infof("Uploading FT model weights to bucket..")
	ftModelWeights := casper.ObjectURI{
		Namespace:  d.Config.ModelObjectStoreURI.Namespace,
		BucketName: d.Config.ModelObjectStoreURI.BucketName,
		ObjectName: d.Config.ModelObjectStoreURI.ObjectName,
	}
	d.uploadModelToObjectStorageHelper(ftModelWeights, d.Config.ZippedModelPath)

	if d.Config.Runtime == constants.PeftTrainingSidecar || d.Config.CohereFineTuneDetails.ServingStrategy == constants.VanillaServingStrategy {
		d.logger.Infof("%s runtime, uploading merged model weights to bucket..", d.Config.Runtime)
		// upload merged model weights to bucket
		mergedModelWeights := casper.ObjectURI{
			Namespace:  d.Config.ModelObjectStoreURI.Namespace,
			BucketName: d.Config.ModelObjectStoreURI.BucketName,
			ObjectName: d.Config.ModelObjectStoreURI.ObjectName + constants.MergedModelWeightZippedFileSuffix,
		}
		d.uploadModelToObjectStorageHelper(mergedModelWeights, d.Config.ZippedMergedModelPath)
	}
}

func (d *TrainingAgent) uploadTrainingMetricsToObjectStorage() {
	response, err := d.FTClient.GetTrainingMetrics()
	if err != nil {
		panic(fmt.Errorf("failed to call /metrics: %+v", err))
	}
	responseBody, err := readResponseBody(response)
	if err != nil {
		panic(fmt.Errorf("failed to read /metrics response: %+v", err))
	}
	if response.StatusCode != http.StatusOK {
		panic(fmt.Errorf("error calling /metrics - StatusCode %d, Response: %s", response.StatusCode, string(responseBody)))
	}

	d.logger.Infof("/metrics - StatusCode: %d, Response: %s", response.StatusCode, string(responseBody))
	filename := "/" + d.Config.TrainingMetricsObjectStoreURI.ObjectName + ".json"

	// Write the string to the file
	err = os.WriteFile(filename, responseBody, os.ModePerm)
	if err != nil {
		panic(fmt.Errorf("unable to write metrics JSON to file: %+v", err))
	}
	d.logger.Infof("Successfully wrote metrics JSON to file %s", filename)

	genaiObjectURI := casper.ObjectURI{
		Namespace:  d.Config.TrainingMetricsObjectStoreURI.Namespace,
		BucketName: d.Config.TrainingMetricsObjectStoreURI.BucketName,
		ObjectName: d.Config.TrainingMetricsObjectStoreURI.ObjectName,
	}
	d.logger.Infof("Pushing metrics object %s in bucket %s under namespace %s", genaiObjectURI.ObjectName, genaiObjectURI.BucketName, genaiObjectURI.Namespace)
	if err := d.Config.OutputObjectStorageDataStore.Upload(filename, genaiObjectURI); err != nil {
		panic(err)
	}
	d.logger.Infof("Successfully uploaded metrics to object storage")
}

func (d *TrainingAgent) terminateTrainingInstance() {
	response, err := d.FTClient.PostTerminate()
	if err != nil {
		d.logger.Warnf("failed to call /terminate: %+v", err)
		return
	}
	d.logger.Infof("/terminate - StatusCode: %d", response.StatusCode)
}

func (d *TrainingAgent) prepareFTDetailsPayload() []byte {
	fineTuneDetails, err := NewFineTuneDetails(&d.Config)
	d.logger.Infof("Finetune details: %+v", fineTuneDetails)
	if err != nil {
		d.logger.Errorf("Error preparing fine tune details: %+v", err)
		panic(err)
	}
	jsonPayload, err := ConvertFTDetailsToJSON(fineTuneDetails)
	if err != nil {
		d.logger.Errorf("Error converting fine tune details to JSON representation: %+v", err)
		panic(err)
	}
	return jsonPayload
}

func (d *TrainingAgent) handleDataError(message string) error {
	isDataError := isDataError(message)
	if isDataError {
		d.logger.Info("Data error detected from training container")
		// Create directory
		err := os.MkdirAll(filepath.Dir(constants.TerminationLogPath), os.ModePerm)
		if err != nil {
			d.logger.Warnf("failed to create directory %s in func handleDataError: %+v", constants.TerminationLogPath, err)
			return err
		}

		// Reset message since data error message from command r ft cohere container is not good enough to directly return to customers
		if strings.Contains(message, constants.CohereCommandRFTDataErrorMessagePrefix) {
			message = "Failed processing dataset, please check dataset if it is a valid format of JSONL"
		}

		// Write message to /dev/termination-log
		err = os.WriteFile(constants.TerminationLogPath, []byte(message), os.ModePerm)
		if err != nil {
			d.logger.Warnf("failed to write message %s to path %s: %+v", message, constants.TerminationLogPath, err)
			return err
		}
		return nil
	}
	d.logger.Info("Not an error from data")
	return nil
}

func (d *TrainingAgent) handlePeftDataError(httpResponse *http.Response, responseBody []byte) {
	/*
	 * For llama3 peft, when it comes to data validation error, peft training container will return error with
	 * status code 422 and message starts with `Data error`
	 */
	if httpResponse.StatusCode == http.StatusUnprocessableEntity {
		var unmarshalledResp Response
		if err := json.Unmarshal(responseBody, &unmarshalledResp); err == nil {
			_ = d.handleDataError(unmarshalledResp.Message) // side error, not the main one from training container which failed training, not throw it out for a better main error determination via logs
		} else {
			d.logger.Warnf("failed to unmarshal /finetune response %s: %+v", string(responseBody), err)
		}
	}
}

func (d *TrainingAgent) zipCohereFTWeights() {
	if d.Config.CohereFineTuneDetails.Strategy == constants.VanillaTrainingStrategy {
		totalLayersNum, err := GetTotalLayerNumberFromModelConfig(filepath.Join(d.Config.ModelDirectory, constants.CohereTrainingConfigPbtxt))
		if err != nil {
			panic(err)
		}

		prefixes := GetLayerNumberPrefixes(totalLayersNum, d.Config.CohereFineTuneDetails.NLastLayers)

		err = zipper.ZipFilesWithPrefixes(d.Config.ModelDirectory, d.Config.ZippedModelPath, prefixes)
		if err != nil {
			d.logger.Errorf("Error zipping files with prefixes: %v", err)
			panic(err)
		} else {
			d.logger.Infof("Successfully zipped files in directory: %s", d.Config.ModelDirectory)
		}
	} else {
		if err := zipper.ZipDirectory(d.Config.ModelDirectory, d.Config.ZippedModelPath); err != nil {
			d.logger.Errorf("Error zipping directory: %v", err)
			panic(err)
		}
		d.logger.Infof("Successfully zipped directory: %s", d.Config.ModelDirectory)
	}
}

/*
*  For llama3 peft, cohere command R TFew FT and cohere command R non-stacked Lora FT, we would zip and store both
*  fine-tuned model weights and merged model weights; Within these 3 FTs, llama3 peft and cohere command R TFew FT
*  right now not support stack serving, store both weights so to be used in future when stacked serving is supported
*  and customers no need to re-train the model.
*
*  The zipped file path would be as below for llama3 peft:
*                      Model Directory                                  Zipped File Path
*  ft-weights:     <PATH_PREFIX>/output/fine-tuned-weights  ->  <PATH_PREFIX>/output/<finetuned_model_suffix>
*  merged-weights: <PATH_PREFIX>/output/base-peft-merged    ->  <PATH_PREFIX>/output/<finetuned_model_suffix>-merged-weight
* ----------------------------------------------------------------------------
*  The zipped file path would be as below for cohere command R TFew FT:
*                      Model Directory                                   Zipped File Path
*  ft-weights:     <PATH_PREFIX>/output/tfew_weights       ->  <PATH_PREFIX>/<finetuned_model_suffix>
*  merged-weights: <PATH_PREFIX>/model/tensorrtllm        ->  <PATH_PREFIX>/<finetuned_model_suffix>-merged-weight
* ----------------------------------------------------------------------------
*  The zipped file path would be as below for cohere command R Non-Stacked Lora FT :
*                      Model Directory                                   Zipped File Path
*  ft-weights:     <PATH_PREFIX>/output/                   ->  <PATH_PREFIX>/<finetuned_model_suffix>
*  merged-weights: <PATH_PREFIX>/model/tensorrtllm        ->  <PATH_PREFIX>/<finetuned_model_suffix>-merged-weight
 */
func (d *TrainingAgent) zipFTWeightsAndMergedWeights(ftWeightsDir string, mergedWeightsDir string) {
	// zip fine tuned model weights
	finetunedModelWeightsDir := filepath.Join(d.Config.ModelDirectory, ftWeightsDir)
	if err := zipper.ZipDirectory(finetunedModelWeightsDir, d.Config.ZippedModelPath); err != nil {
		d.logger.Errorf("Error zipping directory %s : %+v", finetunedModelWeightsDir, err)
		panic(err)
	}
	d.logger.Infof("Successfully zipped directory: %s", finetunedModelWeightsDir)

	// zip merged model weights
	mergedModelWeightsDirectory := filepath.Join(d.Config.ModelDirectory, mergedWeightsDir)
	if err := zipper.ZipDirectory(mergedModelWeightsDirectory, d.Config.ZippedMergedModelPath); err != nil {
		d.logger.Errorf("Error zipping directory %s : %+v", mergedModelWeightsDirectory, err)
		panic(err)
	}
	d.logger.Infof("Successfully zipped directory: %s", mergedModelWeightsDirectory)
}

func (d *TrainingAgent) uploadModelToObjectStorageHelper(objectUrl casper.ObjectURI, uploadedFilePath string) {
	fileInfo, err := os.Stat(uploadedFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			d.logger.Errorf("zipped model weight %s doesn't exist, what happened? : %v", uploadedFilePath, err)
		}
		panic(err)
	}

	d.logger.Infof("Uploading Object %s to bucket %s under namesapce %s", objectUrl.ObjectName, objectUrl.BucketName, objectUrl.Namespace)
	if fileInfo.Size() < int64(BigFileSizeInMB)*int64(casper.MB) {
		if err := d.Config.OutputObjectStorageDataStore.Upload(uploadedFilePath, objectUrl); err != nil {
			panic(err)
		}
	} else {
		if err := d.Config.OutputObjectStorageDataStore.MultipartFileUpload(uploadedFilePath, objectUrl, DefaultUploadChunkSizeInMB, DefaultUploadThreads); err != nil {
			panic(err)
		}
	}
	d.logger.Infof("Successfully uploaded model weight %s to object storage", uploadedFilePath)
}

func (d *TrainingAgent) handleCohereTrainingStatus(unmarshalledResp Response) (bool, error) {
	switch unmarshalledResp.Status {
	case "finished":
		d.logger.Info("Status: Complete")
		return true, nil
	case "idle":
		d.logger.Info("Status: Idle. Checking again in 1 minute...")
		return false, nil
	case "in progress":
		d.logger.Info("Status: In Progress. Checking again in 1 minute...")
		return false, nil
	default:
		d.logger.Errorf("Status: %s", unmarshalledResp.Status)
		return false, fmt.Errorf("error from training, stop here: %+v", unmarshalledResp)
	}
}

func (d *TrainingAgent) handlePeftTrainingStatus(unmarshalledResp Response) (bool, error) {
	// FineTuneStatus from moirai repo:
	// https://bitbucket.oci.oraclecorp.com/projects/GEN/repos/moirai-internal/browse/llm-peft/genai_llm_peft/entrypoint/protocol.py#27-31
	switch unmarshalledResp.Status {
	case "FINISHED":
		d.logger.Info("Status: Finished")
		return true, nil
	case "RUNNING":
		d.logger.Info("Status: Running. Checking again in 1 minute...")
		return false, nil
	case "READY":
		d.logger.Info("Status: READY. Server ready or prepared for running, checking again in 1 minute...")
		return false, nil
	default:
		d.logger.Errorf("Status: %s; Message: %s", unmarshalledResp.Status, unmarshalledResp.Message)
		return false, fmt.Errorf("error from training, stop here: %+v", unmarshalledResp)
	}
}
