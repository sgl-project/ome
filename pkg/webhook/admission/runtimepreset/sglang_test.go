package runtimepreset

import (
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func TestSglangPDPreset_Engine(t *testing.T) {
	g := gomega.NewWithT(t)
	preset := sglangPDPreset()

	g.Expect(preset.EngineConfig).NotTo(gomega.BeNil())
	g.Expect(preset.EngineConfig.Runner).NotTo(gomega.BeNil())

	engine := preset.EngineConfig.Runner.Container
	g.Expect(engine.Name).To(gomega.Equal("ome-container"))
	g.Expect(engine.Image).To(gomega.Equal(sglangEngineImage))
	g.Expect(engine.Command).To(gomega.ContainElement("--disaggregation-mode"))
	g.Expect(engine.Command).To(gomega.ContainElement("prefill"))
	g.Expect(engine.Command).To(gomega.ContainElement("$(MODEL_PATH)"))
	g.Expect(engine.Command).To(gomega.ContainElement("--disaggregation-bootstrap-port"))

	g.Expect(containerPortNames(engine.Ports)).To(gomega.ConsistOf("http", "bootstrap"))
}

func TestSglangPDPreset_Decoder(t *testing.T) {
	g := gomega.NewWithT(t)
	preset := sglangPDPreset()

	decoder := preset.DecoderConfig.Runner.Container
	g.Expect(decoder.Command).To(gomega.ContainElement("decode"))
	// Decoder must not expose the bootstrap port — engine-only in sglang-pd.
	g.Expect(decoder.Command).NotTo(gomega.ContainElement("--disaggregation-bootstrap-port"))
	g.Expect(containerPortNames(decoder.Ports)).To(gomega.ConsistOf("http"))
}

func TestSglangPDPreset_Router(t *testing.T) {
	g := gomega.NewWithT(t)
	preset := sglangPDPreset()

	router := preset.RouterConfig.Runner.Container
	g.Expect(router.Name).To(gomega.Equal("router"))
	g.Expect(router.Image).To(gomega.Equal(sglangRouterImage))
	// Router discovers engine/decoder pods via labels OME already sets
	// on ISVC-owned pods; selector drift here breaks routing.
	g.Expect(router.Args).To(gomega.ContainElement(
		"component=engine,ome.io/inferenceservice=$(INFERENCESERVICE_NAME)"))
	g.Expect(router.Args).To(gomega.ContainElement(
		"component=decoder,ome.io/inferenceservice=$(INFERENCESERVICE_NAME)"))
}

func containerPortNames(ports []corev1.ContainerPort) []string {
	out := make([]string, 0, len(ports))
	for _, p := range ports {
		out = append(out, p.Name)
	}
	return out
}
