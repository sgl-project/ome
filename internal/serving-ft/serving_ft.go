package serving_ft

import (
	cas "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/zipper"
	"fmt"
	"os"
	"path/filepath"
)

const (
	BigFileSizeInMB              = 200
	DefaultDownloadChunkSizeInMB = 128
	DefaultDownloadThreads       = 12
)

// FineTunedServingInit represents an Example application
type FineTunedServingInit struct {
	anotherLogger logging.Interface

	Config Config
}

// NewApplication constructs a new server from the given configuration.
func NewApplication(config *Config) (*FineTunedServingInit, error) {
	return &FineTunedServingInit{
		anotherLogger: config.AnotherLogger,
		Config:        *config,
	}, nil
}

// Start starts the application
func (a *FineTunedServingInit) Start() {
	a.anotherLogger.Infof("Starting finetuned serving init.")

	if a.Config.IsFTWeightsMerged {
		/*
		 * Applied cases:
		 * 1. Vllm Lora FT serving
		 * 2. Cohere Command R TFew FT serving
		 * 3. Cohere Command R non-stacked Lora FT serving
		 */
		a.mergedWeightsFinetuned()
	} else {
		if a.Config.FinetuningStrategy == "vanilla" {
			a.vanillaFinetuned()
		} else if a.Config.FinetuningStrategy == "tfew" {
			a.tfewFinetuned()
		} else if a.Config.FinetuningStrategy == "lora" {
			// Do nothing
			a.anotherLogger.Info("No operations needed for Multi-Lora FT serving in ft serving init")
		} else {
			panic(fmt.Errorf("finetuning strategy %s not supported", a.Config.FinetuningStrategy))
		}
	}

	a.anotherLogger.Infof("Done with finetuned serving init for %s", a.Config.FineTunedWeightURI.ObjectName)
}

func (a *FineTunedServingInit) vanillaFinetuned() {
	// 1. Download the zipped weight file to the model_download_directory
	a.anotherLogger.Infof("Start downloading the finetuned weight %s", a.Config.FineTunedWeightURI.ObjectName)

	err := os.MkdirAll(a.Config.ModelDownloadDirectory, os.ModePerm)
	if err != nil {
		panic(err)
	}

	err = a.Config.CasperDataStore.DownloadBasedOnObjectSize(
		*a.Config.FineTunedWeightURI,
		a.Config.ModelDownloadDirectory,
		true,
		int(BigFileSizeInMB),
		int(DefaultDownloadChunkSizeInMB),
		int(DefaultDownloadThreads))

	if err != nil {
		panic(err)
	}
	fineTunedWeightPath := filepath.Join(a.Config.ModelDownloadDirectory, cas.ExtractPureObjectName(a.Config.FineTunedWeightURI.ObjectName))
	a.anotherLogger.Infof("Finished downloading the finetuned weights %s", a.Config.FineTunedWeightURI.ObjectName)

	// 2. Unzip the finetuned weight to the model_weight_path
	a.anotherLogger.Infof("Start unzipping the finetuned weight %s", a.Config.FineTunedWeightURI.ObjectName)

	extractDirectory := a.Config.ModelWeightDirectory
	err = zipper.Unzip(fineTunedWeightPath, extractDirectory)
	if err != nil {
		panic(err)
	}
	a.anotherLogger.Infof("Finished unzipping the finetuned weight %s", a.Config.FineTunedWeightURI.ObjectName)

	// 3. Delete the downloaded zipped weight
	err = os.Remove(fineTunedWeightPath)
	if err != nil {
		a.anotherLogger.Errorf("Failed to remove %s: %v", fineTunedWeightPath, err)
		// do nothing
	}
}

func (a *FineTunedServingInit) tfewFinetuned() {
	// Setting up the initial config.ini for tfew path
	a.anotherLogger.Infof("Start setting up the initial config for tfew serving")

	configDirectory := filepath.Join(a.Config.ModelWeightDirectory, a.Config.ModelFormat, "1")

	os.MkdirAll(configDirectory, 0777)

	configPath := filepath.Join(configDirectory, "config.ini")

	f, err := os.Create(configPath)
	if err != nil {
		panic(err)
	}

	_, err = f.WriteString(getinitialConfigIni())
	if err != nil {
		f.Close()
		panic(err)
	}
	f.Close()
}

// Setting up the merged weights
// For Vllm LoRA FT serving:
// Loading the LoRA weight on the fly doesn't perform well at prediction time,
// so we only download and load the merged weights until the performance issue fixed

// For Command R TFew/Non-Stacked LoRA FT serving:
// Command R TFew FT not support stack serving right now; while Command R non-stacked LoRA FT serving is a type of FT serving
// using merged weights. So for both of them, we just download and load the merged weights to do the serving.
func (a *FineTunedServingInit) mergedWeightsFinetuned() {
	a.anotherLogger.Infof("Start downloading the merged weights")

	err := os.MkdirAll(a.Config.ModelDownloadDirectory, os.ModePerm)
	if err != nil {
		panic(err)
	}

	// For both LoRA and commandR ft serving:
	// The merged weight will be at the object storage URI + "-merged-weight",
	// The fine-tuned weight will be at the object storage URI,
	// This is for backward compatibility when we could load LoRA weights on the fly or have stack serving supported for commandR
	a.Config.FineTunedWeightURI.ObjectName = fmt.Sprintf("%s-merged-weight", a.Config.FineTunedWeightURI.ObjectName)

	err = a.Config.CasperDataStore.DownloadBasedOnObjectSize(
		*a.Config.FineTunedWeightURI,
		a.Config.ModelDownloadDirectory,
		true,
		int(BigFileSizeInMB),
		int(DefaultDownloadChunkSizeInMB),
		int(DefaultDownloadThreads))

	if err != nil {
		panic(err)
	}

	fineTunedWeightPath := filepath.Join(a.Config.ModelDownloadDirectory, cas.ExtractPureObjectName(a.Config.FineTunedWeightURI.ObjectName))
	a.anotherLogger.Infof("Finished downloading the finetuned weights %s", a.Config.FineTunedWeightURI.ObjectName)

	// 2. Unzip the finetuned weight to the model_weight_path
	a.anotherLogger.Infof("Start unzipping the finetuned weight %s", a.Config.FineTunedWeightURI.ObjectName)

	extractDirectory := a.Config.ModelWeightDirectory
	err = zipper.Unzip(fineTunedWeightPath, extractDirectory)
	if err != nil {
		panic(err)
	}
	a.anotherLogger.Infof("Finished unzipping the finetuned weight %s", a.Config.FineTunedWeightURI.ObjectName)

	// 3. Delete the downloaded zipped weight
	err = os.Remove(fineTunedWeightPath)
	if err != nil {
		a.anotherLogger.Errorf("Failed to remove %s: %v", fineTunedWeightPath, err)
		// do nothing
	}
}

func getinitialConfigIni() string {
	return "[gpt]\nweight_data_type=fp16\n\n[tfew]\ntfew_stack_size=50\n"
}
