package namespace

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestResolveSeparatesWorkloadAndOMENamespaces(t *testing.T) {
	t.Parallel()

	got := mustResolve(t, NewOptions(), "team-a")

	if got.WorkloadNamespace != "team-a" {
		t.Fatalf("WorkloadNamespace = %q, want %q", got.WorkloadNamespace, "team-a")
	}
	if got.OMENamespace != "ome" {
		t.Fatalf("OMENamespace = %q, want %q", got.OMENamespace, "ome")
	}
}

func TestResolveDefaultsAlfredNamespaceToEffectiveOMENamespace(t *testing.T) {
	t.Parallel()

	options := NewOptions()
	options.OMENamespace = "control-plane"

	got := mustResolve(t, options, "team-a")

	if got.AlfredNamespace != "control-plane" {
		t.Fatalf("AlfredNamespace = %q, want %q", got.AlfredNamespace, "control-plane")
	}
}

func TestResolveUsesExplicitAlfredNamespace(t *testing.T) {
	t.Parallel()

	options := NewOptions()
	options.OMENamespace = "control-plane"
	options.AlfredNamespace = "alfred-system"

	got := mustResolve(t, options, "team-a")

	if got.AlfredNamespace != "alfred-system" {
		t.Fatalf("AlfredNamespace = %q, want %q", got.AlfredNamespace, "alfred-system")
	}
}

func TestResolveDefaultsAlfredConfigLocation(t *testing.T) {
	t.Parallel()

	got := mustResolve(t, NewOptions(), "team-a")

	if got.AlfredConfigName != "alfred-config" {
		t.Fatalf("AlfredConfigName = %q, want %q", got.AlfredConfigName, "alfred-config")
	}
	if got.AlfredConfigKey != "config.yaml" {
		t.Fatalf("AlfredConfigKey = %q, want %q", got.AlfredConfigKey, "config.yaml")
	}
}

func TestAddFlagsOMEOverrideFlowsToAlfredNamespace(t *testing.T) {
	t.Parallel()

	options := NewOptions()
	flags := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)
	options.AddFlags(flags)
	if err := flags.Parse([]string{"--ome-namespace", "control-plane"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got := mustResolve(t, options, "team-a")

	if got.WorkloadNamespace != "team-a" {
		t.Fatalf("WorkloadNamespace = %q, want %q", got.WorkloadNamespace, "team-a")
	}
	if got.OMENamespace != "control-plane" {
		t.Fatalf("OMENamespace = %q, want %q", got.OMENamespace, "control-plane")
	}
	if got.AlfredNamespace != "control-plane" {
		t.Fatalf("AlfredNamespace = %q, want %q", got.AlfredNamespace, "control-plane")
	}
}

func TestAddOMEFlagsBindsOnlyOMENamespace(t *testing.T) {
	t.Parallel()

	options := NewOptions()
	flags := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)
	options.AddOMEFlags(flags)
	if got, want := flagNames(flags), []string{"ome-namespace"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("flag names = %v, want %v", got, want)
	}
	if err := flags.Parse([]string{"--ome-namespace", "control-plane"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got := mustResolve(t, options, "team-a")

	if got.OMENamespace != "control-plane" {
		t.Fatalf("OMENamespace = %q, want %q", got.OMENamespace, "control-plane")
	}
	if got.AlfredNamespace != "control-plane" {
		t.Fatalf("AlfredNamespace = %q, want %q", got.AlfredNamespace, "control-plane")
	}
}

func TestAddFlagsOverridesAlfredNamespaceIndependently(t *testing.T) {
	t.Parallel()

	options := NewOptions()
	flags := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)
	options.AddFlags(flags)
	if err := flags.Parse([]string{"--alfred-namespace", "alfred-system"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got := mustResolve(t, options, "team-a")

	if got.OMENamespace != "ome" {
		t.Fatalf("OMENamespace = %q, want %q", got.OMENamespace, "ome")
	}
	if got.AlfredNamespace != "alfred-system" {
		t.Fatalf("AlfredNamespace = %q, want %q", got.AlfredNamespace, "alfred-system")
	}
}

func TestAddAlfredFlagsBindsOnlyAlfredOptions(t *testing.T) {
	t.Parallel()

	options := NewOptions()
	flags := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)
	options.AddAlfredFlags(flags)
	wantNames := []string{"alfred-config-key", "alfred-config-name", "alfred-namespace"}
	if got := flagNames(flags); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("flag names = %v, want %v", got, wantNames)
	}
	args := []string{
		"--alfred-namespace", "alfred-system",
		"--alfred-config-name", "custom-config",
		"--alfred-config-key", "settings.yaml",
	}
	if err := flags.Parse(args); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got := mustResolve(t, options, "team-a")

	if got.OMENamespace != "ome" {
		t.Fatalf("OMENamespace = %q, want %q", got.OMENamespace, "ome")
	}
	if got.AlfredNamespace != "alfred-system" {
		t.Fatalf("AlfredNamespace = %q, want %q", got.AlfredNamespace, "alfred-system")
	}
	if got.AlfredConfigName != "custom-config" {
		t.Fatalf("AlfredConfigName = %q, want %q", got.AlfredConfigName, "custom-config")
	}
	if got.AlfredConfigKey != "settings.yaml" {
		t.Fatalf("AlfredConfigKey = %q, want %q", got.AlfredConfigKey, "settings.yaml")
	}
}

func TestAddFlagsOverridesAlfredConfigName(t *testing.T) {
	t.Parallel()

	options := NewOptions()
	flags := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)
	options.AddFlags(flags)
	if err := flags.Parse([]string{"--alfred-config-name", "custom-config"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got := mustResolve(t, options, "team-a")

	if got.AlfredConfigName != "custom-config" {
		t.Fatalf("AlfredConfigName = %q, want %q", got.AlfredConfigName, "custom-config")
	}
	if got.AlfredConfigKey != "config.yaml" {
		t.Fatalf("AlfredConfigKey = %q, want %q", got.AlfredConfigKey, "config.yaml")
	}
}

func TestAddFlagsOverridesAlfredConfigKey(t *testing.T) {
	t.Parallel()

	options := NewOptions()
	flags := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)
	options.AddFlags(flags)
	if err := flags.Parse([]string{"--alfred-config-key", "settings.yaml"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got := mustResolve(t, options, "team-a")

	if got.AlfredConfigName != "alfred-config" {
		t.Fatalf("AlfredConfigName = %q, want %q", got.AlfredConfigName, "alfred-config")
	}
	if got.AlfredConfigKey != "settings.yaml" {
		t.Fatalf("AlfredConfigKey = %q, want %q", got.AlfredConfigKey, "settings.yaml")
	}
}

func flagNames(flags *pflag.FlagSet) []string {
	var names []string
	flags.VisitAll(func(flag *pflag.Flag) {
		names = append(names, flag.Name)
	})
	return names
}

func TestResolveZeroValueUsesSafeDefaults(t *testing.T) {
	t.Parallel()

	options := &Options{}
	got := mustResolve(t, options, "team-a")

	if got.OMENamespace != DefaultOMENamespace {
		t.Fatalf("OMENamespace = %q, want %q", got.OMENamespace, DefaultOMENamespace)
	}
	if got.AlfredNamespace != DefaultOMENamespace {
		t.Fatalf("AlfredNamespace = %q, want %q", got.AlfredNamespace, DefaultOMENamespace)
	}
	if got.AlfredConfigName != DefaultAlfredConfigName {
		t.Fatalf("AlfredConfigName = %q, want %q", got.AlfredConfigName, DefaultAlfredConfigName)
	}
	if got.AlfredConfigKey != DefaultAlfredConfigKey {
		t.Fatalf("AlfredConfigKey = %q, want %q", got.AlfredConfigKey, DefaultAlfredConfigKey)
	}
}

func TestZeroValueAddFlagsPublishesSafeDefaults(t *testing.T) {
	t.Parallel()

	options := &Options{}
	flags := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)
	options.AddFlags(flags)

	wantDefaults := map[string]string{
		"ome-namespace":      DefaultOMENamespace,
		"alfred-namespace":   "",
		"alfred-config-name": DefaultAlfredConfigName,
		"alfred-config-key":  DefaultAlfredConfigKey,
	}
	for name, want := range wantDefaults {
		flag := flags.Lookup(name)
		if flag == nil {
			t.Fatalf("flag %q is not registered", name)
		}
		if flag.DefValue != want {
			t.Errorf("flag %q default = %q, want %q", name, flag.DefValue, want)
		}
		got, err := flags.GetString(name)
		if err != nil {
			t.Fatalf("GetString(%q) error = %v", name, err)
		}
		if got != want {
			t.Errorf("flag %q value = %q, want %q", name, got, want)
		}
	}
}

func TestAddFlagsRejectsExplicitEmptyValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		arg  string
		want string
	}{
		{name: "OME namespace", arg: "--ome-namespace=", want: "OME namespace"},
		{name: "Alfred namespace", arg: "--alfred-namespace=", want: "Alfred namespace"},
		{name: "Alfred config name", arg: "--alfred-config-name=", want: "Alfred config name"},
		{name: "Alfred config key", arg: "--alfred-config-key=", want: "Alfred config key"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := NewOptions()
			flags := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)
			options.AddFlags(flags)
			if err := flags.Parse([]string{test.arg}); err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			_, err := options.Resolve("team-a")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Resolve() error = %v, want error containing %q", err, test.want)
			}
		})
	}
}

func TestResolveRejectsInvalidKubernetesIdentifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		workload  string
		configure func(*Options)
		want      string
	}{
		{name: "workload namespace", workload: "Team_A", want: "workload namespace"},
		{name: "OME namespace", workload: "team-a", configure: func(o *Options) { o.OMENamespace = "OME_System" }, want: "OME namespace"},
		{name: "Alfred namespace", workload: "team-a", configure: func(o *Options) { o.AlfredNamespace = "alfred/system" }, want: "Alfred namespace"},
		{name: "Alfred config name", workload: "team-a", configure: func(o *Options) { o.AlfredConfigName = "Config_Map" }, want: "Alfred config name"},
		{name: "Alfred config key", workload: "team-a", configure: func(o *Options) { o.AlfredConfigKey = "bad/key" }, want: "Alfred config key"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := NewOptions()
			if test.configure != nil {
				test.configure(options)
			}

			_, err := options.Resolve(test.workload)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Resolve() error = %v, want error containing %q", err, test.want)
			}
		})
	}
}

func mustResolve(t *testing.T, options *Options, workloadNamespace string) Resolved {
	t.Helper()
	got, err := options.Resolve(workloadNamespace)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	return got
}
