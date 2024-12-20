package jobset

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime/framework"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const Name = "JobSet"

type JobSet struct {
	client     client.Client
	restMapper meta.RESTMapper
	scheme     *apiruntime.Scheme
	log        logr.Logger
}

func New(ctx context.Context, c client.Client, _ client.FieldIndexer) (framework.Plugin, error) {
	return &JobSet{
		client:     c,
		restMapper: c.RESTMapper(),
		scheme:     c.Scheme(),
		log:        ctrl.LoggerFrom(ctx).WithValues("pluginName", Name),
	}, nil
}

func (j *JobSet) Name() string {
	return Name
}

func (j *JobSet) Build(ctx context.Context, runtimeJobTemplate client.Object, info *runtime.Info, trainJob *v1beta1.TrainingJob) (client.Object, error) {
	// Todo: Implement JobSet Building logic
	return nil, fmt.Errorf("JobSet building logic is not yet implemented")
}
