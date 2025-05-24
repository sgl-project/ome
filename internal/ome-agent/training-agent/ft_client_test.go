package training_agent

import "testing"

func TestFTClient_NewFTClient(t *testing.T) {
	var config Config
	config.Runtime = "peft"

	client, err := NewFTClient(&config)

	if err != nil {
		t.Errorf("err creating ft client: %v", client)
	}
}

func TestFTClient_NewFineTuneDetails(t *testing.T) {
	var config Config
	config.Runtime = "peft"

	client, err := NewFineTuneDetails(&config)

	if err != nil {
		t.Errorf("err creating ft details: %v", client)
	}
}

func TestFTClient_ConvertFTDetailsToJSON(t *testing.T) {
	payload := &PeftFineTuneDetails{}

	json, err := ConvertFTDetailsToJSON(payload)

	if err != nil {
		t.Errorf("err converting payload: %v", json)
	}
}
