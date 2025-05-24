package ingress

import (
	"testing"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/controllerconfig"

	"github.com/google/go-cmp/cmp"
)

func TestGenerateUrlPath(t *testing.T) {
	type args struct {
		name          string
		namespace     string
		ingressConfig *controllerconfig.IngressConfig
	}

	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "empty path template",
			args: args{
				name:      "model",
				namespace: "user",
				ingressConfig: &controllerconfig.IngressConfig{
					IngressDomain: "my.domain",
				},
			},
			want: "",
		},
		{
			name: "valid path template",
			args: args{
				name:      "model",
				namespace: "user",
				ingressConfig: &controllerconfig.IngressConfig{
					PathTemplate: "/path/to/{{ .Namespace }}/{{ .Name }}",
				},
			},
			want: "/path/to/user/model",
		},
		{
			name: "invalid path template (not parsable)",
			args: args{
				name:      "model",
				namespace: "user",
				ingressConfig: &controllerconfig.IngressConfig{
					UrlScheme:     "https",
					IngressDomain: "my.domain",
					PathTemplate:  "/{{{ .Name }}-{{ .Namespace }}.{{ .IngressDomain }}",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid path template (unknown keys)",
			args: args{
				name:      "model",
				namespace: "user",
				ingressConfig: &controllerconfig.IngressConfig{
					UrlScheme:     "https",
					IngressDomain: "my.domain",
					PathTemplate:  "/{{ .Unknownfield }}/serving/{{ .Namespace }}/{{ .Name }}",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid path template (with host)",
			args: args{
				name:      "model",
				namespace: "user",
				ingressConfig: &controllerconfig.IngressConfig{
					UrlScheme:     "https",
					IngressDomain: "my.domain",
					PathTemplate:  "myhost/serving/{{ .Namespace }}/{{ .Name }}",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid path template (with scheme)",
			args: args{
				name:      "model",
				namespace: "user",
				ingressConfig: &controllerconfig.IngressConfig{
					UrlScheme:     "https",
					IngressDomain: "my.domain",
					PathTemplate:  "http://myhost/serving/{{ .Namespace }}/{{ .Name }}",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateUrlPath(tt.args.name, tt.args.namespace, tt.args.ingressConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateUrlPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Test %q unexpected url (-want +got): %v", tt.name, diff)
			}
		})
	}
}
