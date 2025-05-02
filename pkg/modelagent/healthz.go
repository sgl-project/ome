package modelagent

import (
	"fmt"
	"net/http"
	"os"

	"golang.org/x/sys/unix"
)

type ModelAgentHealthCheck struct {
	modelsRootDir string
}

func NewModelAgentHealthCheck(modelsRootDir string) ModelAgentHealthCheck {
	return ModelAgentHealthCheck{
		modelsRootDir: modelsRootDir,
	}
}

func (h ModelAgentHealthCheck) Name() string {
	return "model-agent-health"
}

func (h ModelAgentHealthCheck) Check(_ *http.Request) error {
	// Check if the model root dir exists
	dirInfo, err := os.Stat(h.modelsRootDir)
	if err != nil {
		return err
	}

	// Check if the model root dir is a directory
	if !dirInfo.IsDir() {
		return fmt.Errorf("%s is not a directory", h.modelsRootDir)
	}

	// Check if the model agent can write to the model root dir
	return unix.Access(h.modelsRootDir, unix.W_OK)
}
