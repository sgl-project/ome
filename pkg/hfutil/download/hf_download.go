package download

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
)

// HuggingFace API endpoints
const (
	// AgreementModelURL is the base URL for model agreements
	AgreementModelURL = "https://huggingface.co/%s"
	// RawModelFileURL is the URL pattern for raw model files
	RawModelFileURL = "https://huggingface.co/%s/raw/%s/%s"
	// LfsModelResolverURL is the URL pattern for LFS model files
	LfsModelResolverURL = "https://huggingface.co/%s/resolve/%s/%s"
	// JSONFileListURL is the URL pattern for model file tree listings
	JSONFileListURL = "https://huggingface.co/api/models/%s/tree/%s/%s"
)

// HFDownloadAgent handles downloading of models from HuggingFace
type HFDownloadAgent struct {
	logger logging.Interface
	Config *Config
	Client http.Client
}

// NewHFDownloadAgent constructs a new HFDownloadAgent.
func NewHFDownloadAgent(config *Config) (*HFDownloadAgent, error) {
	if err := config.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %v", err)
	}
	return &HFDownloadAgent{
		logger: config.Logger,
		Config: config,
		Client: http.Client{},
	}, nil
}

// Start initializes the hf_download process.
func (d *HFDownloadAgent) Start() error {
	d.logger.Infof("Starting hf_download for model: %s, branch: %s, connections: %d",
		d.Config.ModelName, d.Config.Branch, d.Config.NumConnections)

	modelPath := filepath.Join(d.Config.LocalPath, d.Config.ModelName)
	if err := os.MkdirAll(modelPath, os.ModePerm); err != nil {
		d.logger.Errorf("Error creating model directory: %v", err)
		return err
	}

	for attempt := 1; attempt <= d.Config.MaxRetries; attempt++ {
		err := d.processHFFolderTree(modelPath, "")
		if err == nil {
			d.logger.Info("Download completed successfully.")
			return nil
		}
		d.logger.Errorf("Download attempt %d failed: %v", attempt, err)
		time.Sleep(time.Duration(d.Config.RetryInternalInSeconds) * time.Second)
	}
	return fmt.Errorf("maximum retry attempts reached")
}

func (d *HFDownloadAgent) processHFFolderTree(ModelPath string, folderName string) error {
	AgreementURL := fmt.Sprintf(AgreementModelURL, d.Config.ModelName)
	tempFolder := path.Join(ModelPath, folderName, "tmp")
	err := os.MkdirAll(tempFolder, os.ModePerm)
	if err != nil {
		d.logger.Errorf("Error creating temp folder: %v", err)
		return err
	}
	// updated ver: 1.2.5; I cannot clear it if I'm trying to implement resume broken downloads based on a single file
	// defer os.RemoveAll(tempFolder) //delete tmp folder upon returning from this function
	JSONFileListURL := fmt.Sprintf(JSONFileListURL, d.Config.ModelName, d.Config.Branch, folderName)
	var jsonFilesList []HFModel
	for _, file := range jsonFilesList {
		filePath := path.Join(ModelPath, file.Path)
		if file.IsDirectory {
			// Directory handling remains unchanged
			if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
				return err
			}
			//here we should pass the original name with filters, other wise the filter will be applied
			if err := d.processHFFolderTree(ModelPath, file.Path); err != nil {
				return err
			}
		} else {
			// Use NeedsDownload flag to determine if the file should be downloaded
			if file.NeedsDownload {
				if file.IsLFS || NeedsDownload(filePath, file.Size) {
					tempFolder := filepath.Join(ModelPath, "tmp")
					downloadErr := d.downloadFileMultiThread(tempFolder, file.DownloadLink, filePath)
					if downloadErr != nil {
						d.logger.Errorf("Error downloading file with multi-threading: %v", downloadErr)

						return downloadErr
					}
				} else {
					// For smaller files or if not using multi-threading, a single-threaded hf_download can be used
					downloadErr := d.downloadSingleThreaded(file.DownloadLink, filePath)
					if downloadErr != nil {
						d.logger.Errorf("Error downloading file with single-threading: %v", downloadErr)
						return downloadErr
					}
				}
			}
		}
	}
	d.logger.Infof("Getting File Download Files List Tree from: %s", JSONFileListURL)

	req, err := http.NewRequest("GET", JSONFileListURL, nil)
	if err != nil {
		return err
	}
	if d.Config.Token != "" {
		req.Header.Add("Authorization", "Bearer "+d.Config.Token)
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		d.logger.Errorf("Error getting file list: %v", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 403 {
		d.logger.Errorf("You need to manually accept the agreement for this model/dataset: %s on HuggingFace site, No bypass will be implemented", AgreementURL)
	}
	// Read the response body into a byte slice
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		d.logger.Errorf("Error reading response body: %v", err)
		return err
	}

	err = json.Unmarshal(content, &jsonFilesList)
	if err != nil {
		return err
	}
	for i := range jsonFilesList {
		jsonFilesList[i].AppendedPath = path.Join(ModelPath, jsonFilesList[i].Path)
		if jsonFilesList[i].Type == "directory" {
			jsonFilesList[i].IsDirectory = true
			err := os.MkdirAll(path.Join(ModelPath, jsonFilesList[i].Path), os.ModePerm)
			if err != nil {
				return err
			}
			jsonFilesList[i].SkipDownloading = true
			// now if this a folder, this whole function will be called again recursively
			//here we should pass the original name with filters, other wise the filter will be applied

			err = d.processHFFolderTree(ModelPath, jsonFilesList[i].Path) // recursive call
			if err != nil {
				return err
			}
			continue
		}

		jsonFilesList[i].DownloadLink = fmt.Sprintf(RawModelFileURL, d.Config.ModelName, d.Config.Branch, jsonFilesList[i].Path)
		if jsonFilesList[i].Lfs != nil {
			jsonFilesList[i].IsLFS = true
			resolverURL := fmt.Sprintf(LfsModelResolverURL, d.Config.ModelName, d.Config.Branch, jsonFilesList[i].Path)
			getLink, err := d.getRedirectLink(resolverURL)
			if err != nil {
				return err
			}
			jsonFilesList[i].DownloadLink = getLink
		}
	}
	// 2nd loop through the files, checking exists/non-exists
	for i := range jsonFilesList {
		if jsonFilesList[i].IsDirectory || jsonFilesList[i].FilterSkip {
			continue
		}
		filename := jsonFilesList[i].AppendedPath
		if _, err := os.Stat(filename); err == nil {
			// File exists, get its size
			fileInfo, _ := os.Stat(filename)
			size := fileInfo.Size()
			d.logger.Infof("Checking Existing file: %s", jsonFilesList[i].AppendedPath)

			if size == int64(jsonFilesList[i].Size) {
				jsonFilesList[i].SkipDownloading = true
				if jsonFilesList[i].IsLFS {
					if !d.Config.SkipSHA {
						err := VerifyChecksum(jsonFilesList[i].AppendedPath, jsonFilesList[i].Lfs.OidSha265)
						if err != nil {
							err := os.Remove(jsonFilesList[i].AppendedPath)
							if err != nil {
								return err
							}
							jsonFilesList[i].SkipDownloading = false
							d.logger.Warnf("Hash failed for LFS file: %s, will redownload/resume", jsonFilesList[i].AppendedPath)
							return err
						}
						d.logger.Infof("Hash Matched for LFS file: %s", jsonFilesList[i].AppendedPath)
					} else {
						d.logger.Infof("Hash Matching SKIPPED for LFS file: %s", jsonFilesList[i].AppendedPath)
					}
				} else {
					d.logger.Infof("File size matched for non LFS file: %s", jsonFilesList[i].AppendedPath)

				}
			}

		}

	}
	// 3ed loop through the files, downloading missing/failed files
	for i := range jsonFilesList {
		if jsonFilesList[i].IsDirectory || jsonFilesList[i].SkipDownloading || jsonFilesList[i].FilterSkip {
			continue
		}
		if jsonFilesList[i].IsLFS {
			err := d.downloadFileMultiThread(tempFolder, jsonFilesList[i].DownloadLink, jsonFilesList[i].AppendedPath)
			if err != nil {
				return err
			}
			// lfs file, verify by checksum
			d.logger.Infof("Checking SHA256 Hash for LFS file: %s", jsonFilesList[i].AppendedPath)

			if !d.Config.SkipSHA {
				err = VerifyChecksum(jsonFilesList[i].AppendedPath, jsonFilesList[i].Lfs.OidSha265)
				if err != nil {
					err := os.Remove(jsonFilesList[i].AppendedPath)
					if err != nil {
						return err
					}
					// jsonFilesList[i].SkipDownloading = false
					d.logger.Warnf("Hash failed for LFS file: %s, will redownload/resume", jsonFilesList[i].AppendedPath)
					return err
				}
				d.logger.Infof("Hash Matched for LFS file: %s", jsonFilesList[i].AppendedPath)

			} else {
				d.logger.Infof("Hash Matching SKIPPED for LFS file: %s", jsonFilesList[i].AppendedPath)
			}

		} else {
			err = d.downloadSingleThreaded(jsonFilesList[i].DownloadLink, jsonFilesList[i].AppendedPath) // no checksum available for small non-lfs files
			if err != nil {
				return err
			}
			// non-lfs file, verify by size matching
			d.logger.Infof("Checking file size matching: %s", jsonFilesList[i].AppendedPath)

			if _, err := os.Stat(jsonFilesList[i].AppendedPath); err == nil {
				fileInfo, _ := os.Stat(jsonFilesList[i].AppendedPath)
				size := fileInfo.Size()
				if size != int64(jsonFilesList[i].Size) {
					return fmt.Errorf("file size mismatch: %s, filesize: %d. Needed size: %d", jsonFilesList[i].AppendedPath, size, jsonFilesList[i].Size)
				}
			} else {
				return fmt.Errorf("file does not exist: %s", jsonFilesList[i].AppendedPath)
			}
		}
	}
	os.RemoveAll(tempFolder) // by here its safe to delete the temp folder
	return nil
}

func (d *HFDownloadAgent) downloadFileMultiThread(tempFolder, url, outputFileName string) error {
	req, err := d.newAuthenticatedRequest("HEAD", url)
	if err != nil {
		return err
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	contentLength, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil {
		return err
	}

	chunkSize := int64(contentLength / d.Config.NumConnections)
	progress := make(chan int64, d.Config.NumConnections)

	// update 1.2.5; we need to check now, if the tmp folder does exists, if the number of files exists before, matched the number of connection, we can proceed with the logic of resuming
	// Calculate the temp file name pattern.
	baseFileName := path.Base(outputFileName)
	tmpFileNamePattern := filepath.Join(tempFolder, fmt.Sprintf("%s_*.tmp", baseFileName))

	// Use Glob to find all files that match this pattern.
	matches, err := filepath.Glob(tmpFileNamePattern)
	if err != nil {
		d.logger.Errorf("Error finding existing hf_download files: %v", err)
		return err
	}

	// Print the number of matched files.
	// count := len(matches)
	if len(matches) > 0 {
		d.logger.Infof("Found existing incomplete hf_download for the file: %s. Forcing Number of connections to: %d", baseFileName, len(matches))
		d.Config.NumConnections = len(matches)
	}
	wg := &sync.WaitGroup{}

	errChan := make(chan error)

	for i := 0; i < d.Config.NumConnections; i++ {
		start := int64(i) * chunkSize
		end := start + chunkSize

		if i == d.Config.NumConnections-1 {
			end = int64(contentLength)
		}
		wg.Add(1)
		go func(i int, start, end int64) {
			err := d.downloadChunk(tempFolder, path.Base(outputFileName), url, i, start, end, progress)
			if err != nil {
				errChan <- fmt.Errorf("error downloading chunk %d : %s", i, err)
			}

			wg.Done() // prevent panic send on closed channel
		}(i, start, end)
	}
	// Mark the start time of the hf_download
	d.logger.Infof("Start Downloading: %s", outputFileName)

	startTime := time.Now()
	go func() {
		var totalDownloaded int64
		lastPrintTime := startTime.Add(-time.Second)

		rateCheckpoints := make([]struct {
			time  time.Time
			bytes int64
		}, 10)
		for i := range rateCheckpoints {
			rateCheckpoints[i].time = startTime
		}

		for chunkSize := range progress {
			now := time.Now()
			totalDownloaded += chunkSize

			if now.Sub(rateCheckpoints[len(rateCheckpoints)-2].time) >= 1*time.Second {
				for i := 1; i < len(rateCheckpoints); i++ {
					rateCheckpoints[i-1] = rateCheckpoints[i]
				}
			}
			rateCheckpoints[len(rateCheckpoints)-1] = struct {
				time  time.Time
				bytes int64
			}{now, totalDownloaded}

			// Calculate speed in megabytes per second
			elapsed := now.Sub(rateCheckpoints[0].time).Seconds()
			speed := float64(rateCheckpoints[len(rateCheckpoints)-1].bytes-rateCheckpoints[0].bytes) / (1024 * 1024) / elapsed
			if time.Since(lastPrintTime).Seconds() >= 0.1 || totalDownloaded == int64(contentLength) {
				// Inside the loop that tracks hf_download progress
				fmt.Printf("\rDownloading %s Speed: %.2f MB/sec, %.2f%% ", outputFileName, speed, float64(totalDownloaded*100)/float64(contentLength))
				lastPrintTime = time.Now()

			}
		}
	}()

	go func() {
		wg.Wait() // Wait for all downloadChunk to finish
		close(errChan)
	}()

	// Check if there was an error in any of the running routines
	for err := range errChan {
		if err != nil {
			d.logger.Errorf("Error downloading file: %v", err)

			return err
		}
	}

	d.logger.Infof("Merging %s Chunks", outputFileName)

	err = MergeFiles(tempFolder, outputFileName, d.Config.NumConnections)
	if err != nil {
		return err
	}
	d.logger.Infof("Finished Downloading: %s", outputFileName)

	return nil
}

func (d *HFDownloadAgent) downloadSingleThreaded(url, outputFileName string) error {
	outputFile, err := os.Create(outputFileName)

	if err != nil {
		return err
	}
	defer outputFile.Close()

	// Set the authorization header with the Bearer token
	req, err := d.newAuthenticatedRequest("GET", url)
	if err != nil {
		return err // gracefully handle request err
	}
	resp, err := d.executeRequest(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	_, err = io.Copy(outputFile, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func (d *HFDownloadAgent) getRedirectLink(url string) (string, error) {

	req, err := d.newAuthenticatedRequest("GET", url)
	if err != nil {
		return "", err
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if d.Config.Token != "" {
				bearerToken := d.Config.Token
				req.Header.Add("Authorization", "Bearer "+bearerToken)
			}
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode <= 399 {
		redirectURL := resp.Header.Get("Location")
		return redirectURL, nil
	}
	return "", fmt.Errorf("%s", "No redirect found")

}

func (d *HFDownloadAgent) downloadChunk(tempFolder, outputFileName, url string, idx int, start, end int64, progress chan<- int64) error {
	tmpFileName := path.Join(tempFolder, fmt.Sprintf("%s_%d.tmp", outputFileName, idx))
	start, err := AdjustStartByte(tmpFileName, start, end, progress)
	if err != nil {
		return err
	}

	req, err := d.prepareDownloadRequest(url, start, end)
	if err != nil {
		return err
	}

	resp, err := d.executeRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return WriteToFile(resp.Body, tmpFileName, progress)
}

func (d *HFDownloadAgent) prepareDownloadRequest(url string, start, end int64) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating hf_download request: %v", err)
	}
	if d.Config.Token != "" {
		req.Header.Add("Authorization", "Bearer "+d.Config.Token)
	}
	req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", start, end-1))
	return req, nil
}

func (d *HFDownloadAgent) newAuthenticatedRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating %s request: %v", method, err)
	}
	if d.Config.Token != "" {
		req.Header.Add("Authorization", "Bearer "+d.Config.Token)
	}
	return req, nil
}

func (d *HFDownloadAgent) executeRequest(req *http.Request) (*http.Response, error) {
	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request: %v", err)
	}

	if resp.StatusCode == http.StatusUnauthorized && d.Config.Token == "" {
		return nil, fmt.Errorf("repository requires an access token, set via HF_TOKEN")
	}

	return resp, nil
}
