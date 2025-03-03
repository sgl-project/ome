package integration_tests

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOmeAgentIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OME Agent Integration Suite")
}
