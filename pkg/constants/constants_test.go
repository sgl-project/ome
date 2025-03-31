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

func TestPvcName(t *testing.T) {
	name := "test-svc"
	component := "default-component"

	pvcName := PVCName(name, component)

	if pvcName != "pvc-test-svc-default-component" {
		t.Errorf("GetPvcName failed, expected pvc-test-svc-default-component, got %s", pvcName)
	}
}

func TestLongPvcName(t *testing.T) {
	name := "test-svc-test-svc-test-svc-test-svc-test-svc-test-svc-test-svc"
	component := "default-component-default-component-default-component-default-component-default-component"

	pvcName := PVCName(name, component)

	if pvcName != "pvc-est-svc-test-svc-test-svc-mponent-default-component" {
		t.Errorf("GetPvcName failed, expected pvc-est-svc-test-svc-test-svc-mponent-default-component, got %s", pvcName)
	}
}

func TestPvName(t *testing.T) {
	name := "test-svc"
	namespace := "namespace"
	component := "component"

	pvName := PVName(name, namespace, component)

	if pvName != "pv-namespace-test-svc-component" {
		t.Errorf("GetPvcName failed, expected pv-namespace-test-svc-component, got %s", pvName)
	}
}

func TestLongPvName(t *testing.T) {
	name := "test-svc-test-svc-test-svc-test-svc-test-svc-test-svc-test-svc"
	namespace := "namespace-namespace-namespace-namespace-namespace-namespace"
	component := "component-component-component-component-component-component-component"

	pvName := PVName(name, namespace, component)

	if pvName != "pv-espace-namespace-est-svc-test-svc-ponent-component" {
		t.Errorf("GetPvcName failed, expected pv-espace-namespace-est-svc-test-svc-ponent-component, got %s", pvName)
	}
}
