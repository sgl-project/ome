package ingress

import (
	"testing"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateDomainName(t *testing.T) {
	type args struct {
		name          string
		obj           v1.ObjectMeta
		ingressConfig *controllerconfig.IngressConfig
	}

	obj := v1.ObjectMeta{
		Name:      "model",
		Namespace: "test",
		Annotations: map[string]string{
			"annotation": "annotation-value",
		},
		Labels: map[string]string{
			"label": "label-value",
		},
	}

	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "default domain template",
			args: args{
				name: "model",
				obj:  obj,
				ingressConfig: &controllerconfig.IngressConfig{
					IngressDomain:  controllerconfig.DefaultIngressDomain,
					DomainTemplate: controllerconfig.DefaultDomainTemplate,
				},
			},
			want: "model.test.example.com",
		},
		{
			name: "template with dot",
			args: args{
				name: "model",
				obj:  obj,
				ingressConfig: &controllerconfig.IngressConfig{
					IngressDomain:  controllerconfig.DefaultIngressDomain,
					DomainTemplate: "{{ .Name }}.{{ .Namespace }}.{{ .IngressDomain }}",
				},
			},
			want: "model.test.example.com",
		},
		{
			name: "template with annotation",
			args: args{
				name: "model",
				obj:  obj,
				ingressConfig: &controllerconfig.IngressConfig{
					IngressDomain:  controllerconfig.DefaultIngressDomain,
					DomainTemplate: "{{ .Name }}.{{ .Namespace }}.{{ .Annotations.annotation }}.{{ .IngressDomain }}",
				},
			},
			want: "model.test.annotation-value.example.com",
		},
		{
			name: "template with label",
			args: args{
				name: "model",
				obj:  obj,
				ingressConfig: &controllerconfig.IngressConfig{
					IngressDomain:  controllerconfig.DefaultIngressDomain,
					DomainTemplate: "{{ .Name }}.{{ .Namespace }}.{{ .Labels.label }}.{{ .IngressDomain }}",
				},
			},
			want: "model.test.label-value.example.com",
		},
		{
			name: "unknown variable",
			args: args{
				name: "model",
				obj:  obj,
				ingressConfig: &controllerconfig.IngressConfig{
					IngressDomain:  controllerconfig.DefaultIngressDomain,
					DomainTemplate: "{{ .ModelName }}.{{ .Namespace }}.{{ .IngressDomain }}",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid domain name",
			args: args{
				name: "model",
				obj:  obj,
				ingressConfig: &controllerconfig.IngressConfig{
					IngressDomain:  controllerconfig.DefaultIngressDomain,
					DomainTemplate: "{{ .Name }}_{{ .Namespace }}_{{ .IngressDomain }}",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateDomainName(tt.args.name, tt.args.obj, tt.args.ingressConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateDomainName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Test %q unexpected domain (-want +got): %v", tt.name, diff)
			}
		})
	}
}
