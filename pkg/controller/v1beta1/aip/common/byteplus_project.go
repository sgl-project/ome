package common

import (
	"context"
	"strings"
	"time"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type BytePlusProject struct {
	ResourceBase
	Resource *v1beta1.Project
}

func NewBytePlusProject(c client.Client, cs kubernetes.Interface, log logr.Logger, scheme *runtime.Scheme, project *v1beta1.Project) *BytePlusProject {
	return &BytePlusProject{
		ResourceBase: ResourceBase{
			Client:    c,
			Clientset: cs,
			Log:       log,
			Scheme:    scheme,
		},
		Resource: project,
	}
}

func (p *BytePlusProject) Create(ctx context.Context) error {
	creationTime := metav1.NewTime(time.Now())
	p.Resource.Status.ProjectId = strings.ToLower(GenerateId("proj-", p.Resource.UID))
	p.Resource.Status.CreationTime = &creationTime
	p.Resource.Status.LastUpdatedTime = &creationTime

	return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusCreated)
}

func (p *BytePlusProject) Update(ctx context.Context) error {
	p.Resource.Status.ProjectId = strings.ToLower(GenerateId("proj-", p.Resource.UID))
	updateTime := metav1.NewTime(time.Now())
	p.Resource.Status.LastUpdatedTime = &updateTime

	return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusUpdated)
}

func (p *BytePlusProject) Delete(ctx context.Context) error {
	return p.updateCondition(ctx, p.Resource, v1beta1.ProjectStatusArchived)
}
