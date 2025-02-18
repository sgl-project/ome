package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ChatCompletionRequest represents the request payload for the OpenAI-compatible API
type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

// ChatMessage represents a single message in the chat conversation
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Options struct {
	VLLMEndpoint     string
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	IdleTimeout      time.Duration
	Addr             string
	inferenceTimeout time.Duration
}

func DefaultOptions() *Options {
	return &Options{
		VLLMEndpoint:     "http://localhost:8081/health",
		ReadTimeout:      10 * time.Second,
		WriteTimeout:     10 * time.Second,
		IdleTimeout:      120 * time.Second,
		inferenceTimeout: 100 * time.Second,
		Addr:             ":8081",
	}
}

func GetOptions() *Options {
	opt := DefaultOptions()
	flag.StringVar(&opt.VLLMEndpoint, "vllm-endpoint", opt.VLLMEndpoint, "The vLLM health endpoint")
	flag.DurationVar(&opt.ReadTimeout, "read-timeout", opt.ReadTimeout, "The read timeout for the server")
	flag.DurationVar(&opt.WriteTimeout, "write-timeout", opt.WriteTimeout, "The write timeout for the server")
	flag.DurationVar(&opt.IdleTimeout, "idle-timeout", opt.IdleTimeout, "The idle timeout for the server")
	flag.StringVar(&opt.Addr, "addr", opt.Addr, "The address to listen on")
	flag.Parse()
	return opt
}

// Liveness Probe Handler
func livenessHandler(opt *Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isServiceAlive(opt) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, "Liveness check passed")
			log.Println("Liveness check passed")
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintln(w, "Liveness check failed")
			log.Println("Liveness check failed")
		}
	}
}

// Readiness Probe Handler
func readinessHandler(opt *Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isServiceReady(opt) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, "Readiness check passed")
			log.Println("Readiness check passed")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintln(w, "Readiness check failed")
			log.Println("Readiness check failed")
		}
	}
}

// Startup Probe Handler
func startupHandler(opt *Options) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isServiceStarted(opt) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, "Startup check passed")
			log.Println("Startup check passed")
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintln(w, "Startup check failed")
			log.Println("Startup check failed")
		}
	}
}

// Custom logic for checking if the service is alive
func isServiceAlive(opt *Options) bool {
	vllmHealthy := checkEndpoint(opt.VLLMEndpoint)

	if !vllmHealthy {
		log.Printf("vLLM endpoint %s is not responding", opt.VLLMEndpoint)
		return false
	}

	return true
}

func isServiceReady(opt *Options) bool {
	vllmHealthy := checkEndpoint(opt.VLLMEndpoint)

	if !vllmHealthy {
		log.Printf("vLLM endpoint %s is not responding", opt.VLLMEndpoint)
		return false
	}

	return true
}

// Custom logic for checking if the service has started correctly
func isServiceStarted(opt *Options) bool {
	// Check if the Ray head node has started

	vllmStarted := sendInferenceRequest(opt)

	if !vllmStarted {
		log.Printf("Inference request to vLLM endpoint %s failed", opt.VLLMEndpoint)
		return false
	}
	return true
}

// Helper function to send an inference request to the vLLM endpoint
func sendInferenceRequest(opt *Options) bool {
	// Create the request payload
	requestPayload := ChatCompletionRequest{
		Model: "vllm-model",
		Messages: []ChatMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
		},
	}

	// Convert the payload to JSON
	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		log.Printf("Failed to marshal request payload: %v", err)
		return false
	}

	// Send the POST request
	client := &http.Client{
		Timeout: opt.inferenceTimeout, // Set a timeout for the request
	}
	resp, err := client.Post(opt.VLLMEndpoint+"/v1/chat/completions", "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.Printf("Failed to send inference request: %v", err)
		return false
	}
	defer resp.Body.Close()

	// Check if the response status code is 200 OK
	if resp.StatusCode != http.StatusOK {
		log.Printf("Inference request failed with status code: %d", resp.StatusCode)
		return false
	}

	// Optionally, we can check the content of the response body to ensure it's a valid response
	// For simplicity, we're only checking the status code here

	return true
}

// Helper function to check an HTTP endpoint
func checkEndpoint(endpoint string) bool {
	resp, err := http.Get(endpoint + "/health")
	if err != nil {
		log.Printf("Error reaching endpoint %s: %v", endpoint, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Endpoint %s returned non-OK status: %d", endpoint, resp.StatusCode)
		return false
	}

	log.Printf("Endpoint %s is healthy", endpoint)
	return true
}

func main() {
	// Initialize configuration using Viper
	options := GetOptions()
	log.Print("Starting multinode-prober")

	http.HandleFunc("/healthz", livenessHandler(options))
	http.HandleFunc("/readyz", readinessHandler(options))
	http.HandleFunc("/startupz", startupHandler(options))

	http.Handle("/metrics", promhttp.Handler())

	log.Printf("Starting server on port %s", options.Addr)
	server := &http.Server{
		Addr:         options.Addr,
		Handler:      nil,
		ReadTimeout:  options.ReadTimeout,
		WriteTimeout: options.WriteTimeout,
		IdleTimeout:  options.IdleTimeout,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server on %s: %v", options.Addr, err)
	}
}
