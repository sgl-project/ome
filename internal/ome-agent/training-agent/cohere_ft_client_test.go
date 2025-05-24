package training_agent

import "testing"

func TestCohereFTClient_PostTerminate(t *testing.T) {
	client := &Client{}

	var config Config
	config.CohereFineTuneDetails = &CohereFineTuneDetails{}

	cohereclient := newCohereFTClient(&config, client)

	if cohereclient == nil {
		t.Errorf("peftclient was nil: %v", cohereclient)
	}
}
