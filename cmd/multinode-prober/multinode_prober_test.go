package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Mock implementation of the Options struct for testing purposes
func mockOptions() *Options {
	return &Options{
		VLLMEndpoint:     "http://localhost:8081",
		ReadTimeout:      10 * time.Second,
		WriteTimeout:     10 * time.Second,
		IdleTimeout:      120 * time.Second,
		inferenceTimeout: 100 * time.Second,
		Addr:             ":8081",
	}
}

// Mock server for testing endpoint checking
func startMockServer() *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"response": "success"}`))
	})

	return httptest.NewServer(handler)
}

func TestLivenessHandler(t *testing.T) {
	opt := mockOptions()
	server := startMockServer()
	defer server.Close()

	opt.VLLMEndpoint = server.URL

	req, err := http.NewRequest("GET", "/healthz", nil)
	if err != nil {
		t.Fatalf("Could not create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := livenessHandler(opt)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", rr.Code)
	}
}

func TestReadinessHandler(t *testing.T) {
	opt := mockOptions()
	server := startMockServer()
	defer server.Close()

	opt.VLLMEndpoint = server.URL

	req, err := http.NewRequest("GET", "/readyz", nil)
	if err != nil {
		t.Fatalf("Could not create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := readinessHandler(opt)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", rr.Code)
	}
}

func TestStartupHandler(t *testing.T) {
	opt := mockOptions()
	server := startMockServer()
	defer server.Close()

	opt.VLLMEndpoint = server.URL

	req, err := http.NewRequest("GET", "/startupz", nil)
	if err != nil {
		t.Fatalf("Could not create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := startupHandler(opt)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", rr.Code)
	}
}

func TestSendInferenceRequest(t *testing.T) {
	opt := mockOptions()
	server := startMockServer()
	defer server.Close()

	opt.VLLMEndpoint = server.URL

	success := sendInferenceRequest(opt)
	if !success {
		t.Errorf("Expected inference request to succeed, but it failed")
	}
}

func TestCheckEndpoint(t *testing.T) {
	server := startMockServer()
	defer server.Close()

	success := checkEndpoint(server.URL)
	if !success {
		t.Errorf("Expected endpoint to be healthy, but it was not")
	}
}

func TestGetOptions(t *testing.T) {
	options := GetOptions()

	if options.VLLMEndpoint != "http://localhost:8081/health" {
		t.Errorf("Expected default VLLMEndpoint, got %v", options.VLLMEndpoint)
	}
	if options.Addr != ":8081" {
		t.Errorf("Expected default Addr, got %v", options.Addr)
	}
	if options.ReadTimeout != 10*time.Second {
		t.Errorf("Expected default ReadTimeout, got %v", options.ReadTimeout)
	}
}

func TestIsServiceAlive(t *testing.T) {
	opt := mockOptions()
	server := startMockServer()
	defer server.Close()

	opt.VLLMEndpoint = server.URL

	if !isServiceAlive(opt) {
		t.Errorf("Expected service to be alive")
	}
}

func TestIsServiceReady(t *testing.T) {
	opt := mockOptions()
	server := startMockServer()
	defer server.Close()

	opt.VLLMEndpoint = server.URL

	if !isServiceReady(opt) {
		t.Errorf("Expected service to be ready")
	}
}

func TestIsServiceStarted(t *testing.T) {
	opt := mockOptions()
	server := startMockServer()
	defer server.Close()

	opt.VLLMEndpoint = server.URL

	if !isServiceStarted(opt) {
		t.Errorf("Expected service to be started")
	}
}

func TestMainHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatalf("Could not create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := promhttp.Handler()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", rr.Code)
	}
}
