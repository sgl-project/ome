package hf_download_agent

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/casper"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

const (
	base                       = "https://huggingface.co"
	defaultDownloadThreads     = 8
	defaultUploadChunkSizeInMB = 25
	defaultUploadThreads       = 8
)

type HFDownloadAgent struct {
	logger logging.Interface

	Config HFConfig

	Client http.Client
}

// NewHFDownloadAgent constructs a new model download agent by the given configuration for HF
func NewHFDownloadAgent(config *HFConfig) (*HFDownloadAgent, error) {
	if err := config.ValidateHFConfig(); err != nil {
		return nil, fmt.Errorf("HF download agent configuration invalid: %v", err)
	}

	client := http.Client{}
	return &HFDownloadAgent{
		logger: config.Logger,
		Config: *config,
		Client: client,
	}, nil
}

func (d *HFDownloadAgent) Start() {
	d.logger.Infof("Start model download agent for HF model %s from commit %s", d.Config.ModelName, d.Config.DownloadCommit)

	// 1. Prepare all download links
	var links []string
	modelFilesJsonTreeBaseURLSuffix := fmt.Sprintf("/api/models/%s/tree/%s", d.Config.ModelName, d.Config.DownloadCommit)
	links, err := d.getDownloadLinks(modelFilesJsonTreeBaseURLSuffix, "", links)
	if err != nil {
		panic(err)
	}
	d.logger.Infof("Total number of model files which require to be downloaded: %d", len(links))

	// 2. Download files and upload to object store
	results := d.ImportWithMultiThreads(links, defaultDownloadThreads)
	for result := range results {
		if result.error != nil {
			panic(result.error)
		}
	}
	d.logger.Infof("Done with model download agent for model %s", d.Config.ModelName)
}

func (d *HFDownloadAgent) getDownloadLinks(modelFilesJsonTreeBaseURLSuffix string, directoryPath string, links []string) ([]string, error) {
	// 1. Construct the url for model files json tree page, e.g. https://huggingface.co/api/models/bigscience/bloom/tree/main
	if len(directoryPath) > 0 {
		modelFilesJsonTreeBaseURLSuffix = fmt.Sprintf("/api/models/%s/tree/%s/%s", d.Config.ModelName, d.Config.DownloadCommit, directoryPath)
	}
	url := base + modelFilesJsonTreeBaseURLSuffix
	d.logger.Infof("Current model files json tree URL is %s", url)

	// 2. Make request to get the json tree page
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return links, fmt.Errorf("error making the request to model files url %s: %+v", url, err)
	}
	if len(d.Config.Token) > 0 {
		request.Header.Add("authorization", fmt.Sprintf("Bearer %s", d.Config.Token))
	}
	response, err := d.Client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// 3. Decode the content of json tree page
	var files []interface{}
	err = json.NewDecoder(response.Body).Decode(&files)
	if err != nil {
		return nil, err
	}
	d.logger.Infof("The number of items under %s is %d", url, len(files))

	// 4. Iterate over each model file item included in json tree, and then resolve the download link per each
	for _, file := range files {
		fileMap, isMap := file.(map[string]interface{})
		if !isMap {
			return links, fmt.Errorf("the json tree entry of the model file is invalid: %s", fileMap)
		}
		filePath, pathExist := fileMap["path"].(string)
		if !pathExist {
			return links, fmt.Errorf("no path exists for model file %+v", file)
		}

		if fileMap["type"].(string) == "file" {
			// an example for download link: https://huggingface.co/bigscience/bloom/resolve/main/config.json
			link := fmt.Sprintf("%s/%s/resolve/%s/%s", base, d.Config.ModelName, d.Config.DownloadCommit, filePath)
			links = append(links, link)
		} else if fileMap["type"].(string) == "directory" {
			d.logger.Infof("Encounter a directory model item %s", filePath)
			links, err = d.getDownloadLinks(modelFilesJsonTreeBaseURLSuffix, filePath, links)
			if err != nil {
				return links, err
			}
		} else {
			return links, fmt.Errorf("unsupported file type %s", fileMap["type"].(string))
		}
	}
	return links, nil
}

type HFDownloadAgentResult struct {
	downloadedFileName string
	error              error
}

func (d *HFDownloadAgent) ImportWithMultiThreads(links []string, numOfThreads int) chan *HFDownloadAgentResult {
	d.logger.Infof("The object storage client endpoint is %s", d.Config.CasperDataStore.Client.Endpoint())
	linkChan := d.prepareLinksChannel(links)
	results := make(chan *HFDownloadAgentResult, len(links))

	var wg sync.WaitGroup
	wg.Add(numOfThreads)

	for i := 0; i < numOfThreads; i++ {
		go func(i int) {
			d.importFiles(linkChan, results)
			wg.Done()
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

func (d *HFDownloadAgent) importFiles(links <-chan string, results chan<- *HFDownloadAgentResult) {
	for link := range links {
		fileName := strings.ReplaceAll(link, fmt.Sprintf("%s/%s/resolve/%s/", base, d.Config.ModelName, d.Config.DownloadCommit), "")
		result := HFDownloadAgentResult{
			downloadedFileName: fileName,
		}

		// Make request to download link
		request, err := http.NewRequest("GET", link, nil)
		if err != nil {
			result.error = err
		}
		// Set token if exists
		if len(d.Config.Token) > 0 {
			request.Header.Add("authorization", fmt.Sprintf("Bearer %s", d.Config.Token))
		}
		resp, err := d.Client.Do(request)
		if err != nil {
			d.logger.Errorf("Failed to download the file %s using link %s: %+v", fileName, link, err)
			result.error = err
		}
		defer resp.Body.Close()

		// Upload downloaded stream to object storage
		uploadedObjectURI := casper.ObjectURI{
			Namespace:  d.Config.ObjectStoreURI.Namespace,
			BucketName: d.Config.ObjectStoreURI.BucketName,
			ObjectName: fmt.Sprintf("%s/%s", d.Config.InternalModelName, fileName),
		}
		err = d.Config.CasperDataStore.MultipartStreamUpload(resp.Body, uploadedObjectURI, defaultUploadChunkSizeInMB, defaultUploadThreads)
		if err != nil {
			d.logger.Errorf("Failed to upload the object %s to bucket %s under namespace %s", uploadedObjectURI.ObjectName, uploadedObjectURI.BucketName, uploadedObjectURI.Namespace)
			result.error = err
		}
		results <- &result
	}
}

func (d *HFDownloadAgent) prepareLinksChannel(links []string) chan string {
	linkChan := make(chan string, len(links))
	go func() {
		defer func() {
			close(linkChan)
		}()
		for _, link := range links {
			linkChan <- link
		}
	}()
	return linkChan
}
