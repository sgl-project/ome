package training_agent

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/sgl-project/sgl-ome/pkg/constants"
)

const (
	DecoderLayersStr = "decoder_layers"
	StringValueStr   = "string_value"
)

func GetTotalLayerNumberFromModelConfig(configPbtxtFilePath string) (int, error) {
	fileBytes, err := os.ReadFile(configPbtxtFilePath)
	if err != nil {
		return 0, err
	}

	text := string(fileBytes)
	decoderLayersIndex := strings.Index(text, DecoderLayersStr)
	if decoderLayersIndex == -1 {
		return 0, fmt.Errorf("the config.pbtxt file doesn't contain %s", DecoderLayersStr)
	}

	targetText := text[(decoderLayersIndex + len(DecoderLayersStr)):]

	stringValueIndex := strings.Index(targetText, StringValueStr)
	if stringValueIndex == -1 {
		return 0, fmt.Errorf("the remaining text in config.pbtxt file doesn't contain %s", StringValueStr)
	}

	targetText = targetText[(stringValueIndex + len(StringValueStr)):]

	var left int = 0
	var right int = 0
	var count int = 0
	for pos, char := range targetText {
		if char == '"' {
			if count == 0 {
				left = pos + 1
				count++
			} else if count == 1 {
				right = pos
				count++
			}

			if count == 2 {
				break
			}
		}
	}

	if count != 2 {
		return 0, fmt.Errorf("the remaining text in config.pbtxt file doesn't contain the total layer number")
	}

	totalLayerNumberStr := targetText[left:right]

	totalNumber, err := strconv.Atoi(totalLayerNumberStr)
	if err != nil {
		return 0, fmt.Errorf("the layer number parsing failed, text: %s", totalLayerNumberStr)
	}

	return totalNumber, nil
}

func GetLayerNumberPrefixes(totalLayerNumber, nLastLayers int) []string {
	prefixes := make([]string, 0)

	firstLayer := totalLayerNumber - nLastLayers
	for i := firstLayer; i < totalLayerNumber; i++ {
		prefix := fmt.Sprintf("1/model.layers.%d.", i)
		prefixes = append(prefixes, prefix)
	}

	return prefixes
}

func readResponseBody(response *http.Response) ([]byte, error) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// TODO: Work with Cohere to use /status API status code to determine the data error
func isDataError(message string) bool {
	dataErrorPrefixes := []string{constants.CohereFaxFTDataErrorMessagePrefix, constants.CohereCommandRFTDataErrorMessagePrefix, constants.PeftDataErrorMessagePrefix}

	for _, item := range dataErrorPrefixes {
		if strings.Contains(message, item) {
			return true
		}
	}
	return false
}
