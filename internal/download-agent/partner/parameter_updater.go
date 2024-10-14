package partner_download_agent

import (
	"os"
	"strings"
)

// AddAndUpdateModelParameters add and update parameters in config.pbtxt file within model
// Please bear with me for this brute force solution.
// It is just to unblock the progress as quick as possible and just used temporarily; Later we will require Cohere to add/update all these parameters for us
func AddAndUpdateModelParameters(stringConfig string, objectName string) []byte {

	// Update model_checkpoint_path
	oldModelCPPath := "/mnt/models/fastertransformer/1"
	newModelCPPath := "/opt/ml/model/fastertransformer/1"
	stringConfig = strings.Replace(stringConfig, oldModelCPPath, newModelCPPath, 1)

	tmpPath := "/tmp/config_update"
	err := os.WriteFile(tmpPath, []byte(stringConfig), os.ModePerm)
	if err != nil {
		os.RemoveAll(tmpPath)
		panic(err)
	}
	file, err := os.OpenFile(tmpPath, os.O_APPEND|os.O_WRONLY, 0600)

	// Add tokenizer parameter
	var tokenizerParameters string
	if strings.Contains(objectName, "embed") || strings.Contains(objectName, "e355m") {
		// Only patch this one specific embed model. This is ugly, but need this for urgent fix.
		if strings.Contains(objectName, "e355m-3.1.0-ft-triton") {
			tokenizerParameters = "parameters {\n    key: \"tokenizer\"\n    value: {\n        string_value: \"50k\"\n    } \n}\n"
			if _, err = file.WriteString(tokenizerParameters); err != nil {
				panic(err)
			}
		}
	}

	if isLegacyModels(objectName) {
		tokenizerParameters = "parameters {\n    key: \"tokenizer\"\n    value: {\n        string_value: \"75k+bos+eos+eop\"\n    } \n}\n"
		if _, err = file.WriteString(tokenizerParameters); err != nil {
			panic(err)
		}

		// Add eos_token parameter
		sequenceTokenParameters := "parameters {\n    key: \"eos_token\"\n    value: {\n        string_value: \"<EOS_TOKEN>\"\n    } \n}\n"
		if _, err = file.WriteString(sequenceTokenParameters); err != nil {
			panic(err)
		}

		// Add batching_config parameter
		if strings.Contains(objectName, "6b") || strings.Contains(objectName, "6B") {
			batchConfigParameter := "parameters {\n    key: \"batching_config\"\n    value: {\n       string_value: \"gpt-6b-ctx4096-tkzr75k\"\n    }\n}\n"
			if _, err = file.WriteString(batchConfigParameter); err != nil {
				panic(err)
			}
		} else if strings.Contains(objectName, "52b") || strings.Contains(objectName, "52B") {
			batchConfigParameter := "parameters {\n    key: \"batching_config\"\n    value: {\n       string_value: \"gpt-52b-tp4-ctx4096-tkzr75k\"\n    }\n}\n"
			if _, err = file.WriteString(batchConfigParameter); err != nil {
				panic(err)
			}
		}
	}

	updatedConfig, err := os.ReadFile(tmpPath)
	if err != nil {
		panic(err)
	}

	// Remove temp file
	err = os.RemoveAll(tmpPath)
	if err != nil {
		panic(err)
	}
	return updatedConfig
}

func isLegacyModels(objectName string) bool {
	return strings.Contains(objectName, "command_6b_v14.2.0_llh_tokenizers_jul23_lmxjd8k0_pref") ||
		strings.Contains(objectName, "command_52b_v14.2.0_llh_tokenizers_jul23_md080abg_pref") ||
		strings.Contains(objectName, "command_52b_int8_v15.5.0") ||
		strings.Contains(objectName, "command_52b_v15.0.0_pref") ||
		strings.Contains(objectName, "command_52b_v15.5.0_pref") ||
		strings.Contains(objectName, "command_52b_v15.0.0_op") ||
		strings.Contains(objectName, "command_52b_v14.6.0_op") ||
		strings.Contains(objectName, "command_6b_v13.2.0_may23_m0h8jzgl") ||
		strings.Contains(objectName, "command_6b_v14.0.0_jun23_xy12qs19_pref") ||
		strings.Contains(objectName, "command_52b_v14.10.0_aug23_uldnqsvc_pref") ||
		strings.Contains(objectName, "command_52b_v13.2.0_sep12")
}
