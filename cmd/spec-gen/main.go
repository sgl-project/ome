package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	omev1beta1 "github.com/sgl-project/ome/pkg/openapi"
	"k8s.io/klog/v2"
	"k8s.io/kube-openapi/pkg/common"
	spec "k8s.io/kube-openapi/pkg/validation/spec"
)

// Generate OpenAPI spec definitions for InferenceService Resource
func main() {
	if len(os.Args) <= 1 {
		klog.Fatal("Supply a version")
	}
	version := os.Args[1]
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	oAPIDefs := omev1beta1.GetOpenAPIDefinitions(func(name string) spec.Ref {
		return spec.MustCreateRef("#/definitions/" + common.EscapeJsonPointer(swaggify(name)))
	})
	defs := spec.Definitions{}
	for defName, val := range oAPIDefs {
		defs[swaggify(defName)] = val.Schema
	}
	swagger := spec.Swagger{
		SwaggerProps: spec.SwaggerProps{
			Swagger:     "2.0",
			Definitions: defs,
			Paths:       &spec.Paths{Paths: map[string]spec.PathItem{}},
			Info: &spec.Info{
				InfoProps: spec.InfoProps{
					Title:       "OME",
					Description: "Python SDK for OME",
					Version:     version,
				},
			},
		},
	}
	jsonBytes, err := json.MarshalIndent(swagger, "", "  ")
	if err != nil {
		klog.Fatal(err.Error())
	}
	fmt.Println(string(jsonBytes))
}

func swaggify(name string) string {
	name = strings.ReplaceAll(name, "github.com/sgl-project/ome/pkg/apis/ome/", "")
	name = strings.ReplaceAll(name, "./pkg/apis/ome/", "")
	name = strings.ReplaceAll(name, "knative.dev/pkg/apis/duck/v1.", "knative/")
	name = strings.ReplaceAll(name, "knative.dev/pkg/apis.", "knative/")
	name = strings.ReplaceAll(name, "k8s.io/api/core/", "")
	name = strings.ReplaceAll(name, "k8s.io/apimachinery/pkg/apis/meta/", "")
	name = strings.ReplaceAll(name, "k8s.io/apimachinery/pkg/api/", "")
	name = strings.ReplaceAll(name, "/", ".")
	return name
}
