package constants

import "testing"

func TestGetPvcName(t *testing.T) {
	tjobName := "test-trainjob"
	tjobNamespace := "default"
	baseModelName := "model"

	pvcName := GetPvcName(tjobName, tjobNamespace, baseModelName)

	if pvcName != "pvc-default-model-test-trainjob" {
		t.Errorf("GetPvcName failed, expected pvc-default-model-test-trainjob, got %s", pvcName)
	}
}

func TestGetLongPvcName(t *testing.T) {
	tjobName := "test-trainjob-test-trainjob-test-trainjob-test-trainjob"
	tjobNamespace := "default-default-default-default-default-default"
	baseModelName := "model-model-model-model-model-model"

	pvcName := GetPvcName(tjobName, tjobNamespace, baseModelName)

	if pvcName != "pvc-t-default-default-default-l-model-model-model-model-st-trainjob-test-trainjob" {
		t.Errorf("GetPvcName failed, expected pvc-t-default-default-default-l-model-model-model-model-st-trainjob-test-trainjob, got %s", pvcName)
	}
}

func TestGetPvName(t *testing.T) {
	tjobName := "test-trainjob"
	tjobNamespace := "default"
	baseModelName := "model"

	pvName := GetPvName(tjobName, tjobNamespace, baseModelName)

	if pvName != "pv-default-model-test-trainjob" {
		t.Errorf("GetPvcName failed, expected pv-default-model-test-trainjob, got %s", pvName)
	}
}

func TestGetLongPvName(t *testing.T) {
	tjobName := "test-trainjob-test-trainjob-test-trainjob-test-trainjob"
	tjobNamespace := "default-default-default-default-default-default"
	baseModelName := "model-model-model-model-model-model"

	pvName := GetPvName(tjobName, tjobNamespace, baseModelName)

	if pvName != "pv--default-default-odel-model-model-ob-test-trainjob" {
		t.Errorf("GetPvcName failed, expected pv--default-default-odel-model-model-ob-test-trainjob, got %s", pvName)
	}
}
