package utils

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
)

func GetFineTunedModelName(trainingJobName string) string {
	return trainingJobName[len(constants.TrainingJobNamePrefix):]
}
