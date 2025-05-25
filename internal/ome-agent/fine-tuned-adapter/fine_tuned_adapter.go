package fine_tuned_adapter

import (
	"os"
	"path/filepath"

	"github.com/sgl-project/sgl-ome/pkg/casper"
	"github.com/sgl-project/sgl-ome/pkg/logging"
	"github.com/sgl-project/sgl-ome/pkg/zipper"
)

const (
	BigFileSizeInMB              = 200
	DefaultDownloadChunkSizeInMB = 128
	DefaultDownloadThreads       = 12
)

type FineTunedAdapter struct {
	logger logging.Interface
	Config Config
}

// NewFineTunedAdapter constructs a fine-tuned weight adapter from the given configuration.
func NewFineTunedAdapter(config *Config) (*FineTunedAdapter, error) {
	return &FineTunedAdapter{
		logger: config.AnotherLogger,
		Config: *config,
	}, nil
}

func (m *FineTunedAdapter) Start() error {
	m.logger.Infof("Start downloading the fine-tuned weight")

	err := os.MkdirAll(m.Config.ZippedFineTunedWeightDirectory, os.ModePerm)
	if err != nil {
		m.logger.Errorf("Failed to create zipped fine-tuned model directory %s", m.Config.ZippedFineTunedWeightDirectory)
		return err
	}

	// 1. Download the fine-tuned weight
	err = m.Config.ObjectStorageDataStore.DownloadBasedOnObjectSize(
		*m.Config.FineTunedWeightURI,
		m.Config.ZippedFineTunedWeightDirectory,
		true,
		int(BigFileSizeInMB),
		int(DefaultDownloadChunkSizeInMB),
		int(DefaultDownloadThreads),
	)
	if err != nil {
		return err
	}

	fineTunedWeightPath := filepath.Join(m.Config.ZippedFineTunedWeightDirectory, casper.ExtractPureObjectName(m.Config.FineTunedWeightURI.ObjectName))
	m.logger.Infof("Finished downloading the fine-tuned weight %s", m.Config.FineTunedWeightURI.ObjectName)

	// 2. Unzip the fine-tuned weight to the required path
	m.logger.Infof("Start unzipping the fine-tuned weight %s", m.Config.FineTunedWeightURI.ObjectName)
	err = zipper.Unzip(fineTunedWeightPath, m.Config.UnzippedFineTunedWeightDirectory)
	if err != nil {
		return err
	}
	m.logger.Infof("Finished unzipping the fine-tuned weight %s", m.Config.FineTunedWeightURI.ObjectName)

	// 3. Delete the downloaded zipped weight
	err = os.Remove(fineTunedWeightPath)
	if err != nil {
		m.logger.Errorf("Failed to remove %s: %v", fineTunedWeightPath, err)
		// do nothing
	}

	return nil
}
