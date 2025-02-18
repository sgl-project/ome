package merged_finetuned_adapter

import (
	"fmt"
	"os"
	"path/filepath"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/zipper"
)

const (
	BigFileSizeInMB              = 200
	DefaultDownloadChunkSizeInMB = 128
	DefaultDownloadThreads       = 12
)

type MergedFinetunedAdapter struct {
	logger logging.Interface
	Config Config
}

// NewMergedFinetunedAdapter constructs a new merged finetuned weights adapter from the given configuration.
func NewMergedFinetunedAdapter(config *Config) (*MergedFinetunedAdapter, error) {
	return &MergedFinetunedAdapter{
		logger: config.AnotherLogger,
		Config: *config,
	}, nil
}

func (m *MergedFinetunedAdapter) Start() error {
	m.logger.Infof("Start downloading the merged finetuned weights.")

	err := os.MkdirAll(m.Config.zippedFinetunedModelDirectory, os.ModePerm)
	if err != nil {
		return err
	}

	m.Config.FineTunedWeightURI.ObjectName = fmt.Sprintf("%s-merged-weight", m.Config.FineTunedWeightURI.ObjectName)

	err = m.Config.ObjectStorageDataStore.DownloadBasedOnObjectSize(
		*m.Config.FineTunedWeightURI,
		m.Config.zippedFinetunedModelDirectory,
		true,
		int(BigFileSizeInMB),
		int(DefaultDownloadChunkSizeInMB),
		int(DefaultDownloadThreads),
	)
	if err != nil {
		return err
	}

	fineTunedWeightPath := filepath.Join(m.Config.unzippedFinetunedModelDirectory, casper.ExtractPureObjectName(m.Config.FineTunedWeightURI.ObjectName))
	m.logger.Infof("Finished downloading the finetuned weights %s", m.Config.FineTunedWeightURI.ObjectName)

	// 2. Unzip the finetuned weight to the model_weight_path
	m.logger.Infof("Start unzipping the finetuned weight %s", m.Config.FineTunedWeightURI.ObjectName)
	extractDirectory := m.Config.unzippedFinetunedModelDirectory
	err = zipper.Unzip(fineTunedWeightPath, extractDirectory)
	if err != nil {
		return err
	}

	m.logger.Infof("Finished unzipping the finetuned weight %s", m.Config.FineTunedWeightURI.ObjectName)
	// 3. Delete the downloaded zipped weight
	err = os.Remove(fineTunedWeightPath)
	if err != nil {
		m.logger.Errorf("Failed to remove %s: %v", fineTunedWeightPath, err)
		// do nothing
	}

	return nil
}
