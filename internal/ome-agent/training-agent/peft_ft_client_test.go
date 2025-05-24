package training_agent

import "testing"

func TestPeftFTClient_PostTerminate(t *testing.T) {
	client := &Client{}

	var config Config
	config.PeftFineTuneDetails = &PeftFineTuneDetails{}

	peftclient := newPeftFTClient(&config, client)

	if peftclient == nil {
		t.Errorf("peftclient was nil: %v", peftclient)
	}
}
