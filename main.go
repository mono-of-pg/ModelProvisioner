package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v2"
)

// Config holds the configuration from the ConfigMap
type Config struct {
	Litellm struct {
		URL string `yaml:"url"`
	} `yaml:"litellm"`
	Backends []struct {
		Name string `yaml:"name"`
		URL  string `yaml:"url"`
	} `yaml:"backends"`
}

// CurrentModelEntry represents a model entry fetched from LiteLLM's /model/info
type CurrentModelEntry struct {
	ModelName     string `json:"model_name"`
	LitellmParams struct {
		Model   string `json:"model"`
		ApiBase string `json:"api_base"`
	} `json:"litellm_params"`
	ModelInfo struct {
		ID string `json:"id"`
	} `json:"model_info"`
}

// DesiredModelEntry represents a model entry to be added to LiteLLM
type DesiredModelEntry struct {
	ModelName     string `json:"model_name"`
	LitellmParams struct {
		Model   string `json:"model"`
		ApiBase string `json:"api_base"`
		ApiKey  string `json:"api_key"`
	} `json:"litellm_params"`
}

// DeleteModelPayload represents the payload for deleting a model from LiteLLM
type DeleteModelPayload struct {
	ID string `json:"id"`
}

// **Debug Mode Configuration**
var debugMode bool

func init() {
	if os.Getenv("DEBUG") == "true" {
		debugMode = true
	}
}

// obfuscateKey obfuscates the API key for logging
func obfuscateKey(key string) string {
	if len(key) < 6 {
		return "REDACTED"
	}
	return key[:4] + "..REDACTED.." + key[len(key)-2:]
}

// readConfig reads and parses the ConfigMap YAML file
func readConfig(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// getModels queries a backend's /models endpoint
func getModels(backendURL, apiKey string) ([]string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", backendURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if debugMode {
		obfuscatedKey := obfuscateKey(apiKey)
		log.Printf("Fetching models: URL=%s, Method=GET, Headers=map[Authorization:Bearer %s]", backendURL+"/models", obfuscatedKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if debugMode {
		log.Printf("Response body from %s/models: %s", backendURL, string(body))
	}
	if resp.StatusCode != 200 {
		if debugMode {
			log.Printf("Error fetching models: Status=%d, Body=%s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("non-200 status: %s, body: %s", resp.Status, string(body))
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}
	models := make([]string, len(result.Data))
	for i, m := range result.Data {
		models[i] = m.ID
	}
	if debugMode {
		log.Printf("Fetched models from %s: %v", backendURL, models)
	}
	return models, nil
}

// getCurrentModels fetches the current model entries from LiteLLM's /model/info
func getCurrentModels(litellmURL, litellmApiKey string) ([]CurrentModelEntry, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", litellmURL+"/model/info", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+litellmApiKey)
	if debugMode {
		obfuscatedKey := obfuscateKey(litellmApiKey)
		log.Printf("Fetching current models: URL=%s, Method=GET, Headers=map[Authorization:Bearer %s]", litellmURL+"/model/info", obfuscatedKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if debugMode {
		log.Printf("Response body from %s/model/info: %s", litellmURL, string(body))
	}
	if resp.StatusCode != 200 {
		if debugMode {
			log.Printf("Error fetching current models: Status=%d, Body=%s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("non-200 status: %s, body: %s", resp.Status, string(body))
	}
	var result struct {
		Data []CurrentModelEntry `json:"data"`
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}
	if debugMode {
		for _, m := range result.Data {
			log.Printf("Current model: %s from %s", m.ModelName, m.LitellmParams.ApiBase)
		}
	}
	return result.Data, nil
}

// addModel adds a new model deployment to LiteLLM via /model/new
func addModel(litellmURL, litellmApiKey string, entry DesiredModelEntry) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if debugMode {
		obfuscatedKey := obfuscateKey(litellmApiKey)
		log.Printf("Adding model: URL=%s, Method=POST, Headers=map[Content-Type:application/json Authorization:Bearer %s], Payload=%s", litellmURL+"/model/new", obfuscatedKey, string(payload))
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", litellmURL+"/model/new", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+litellmApiKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		if debugMode {
			log.Printf("Error adding model %s: Status=%d, Body=%s", entry.ModelName, resp.StatusCode, string(body))
		}
		return fmt.Errorf("non-200 status: %s, body: %s", resp.Status, string(body))
	}
	if debugMode {
		log.Printf("Successfully added model %s: Status=200, Body=%s", entry.ModelName, string(body))
	}
	return nil
}

// removeModel removes a model deployment from LiteLLM via /model/delete
func removeModel(litellmURL, litellmApiKey string, id string) error {
	payload := DeleteModelPayload{ID: id}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if debugMode {
		obfuscatedKey := obfuscateKey(litellmApiKey)
		log.Printf("Removing model: URL=%s, Method=POST, Headers=map[Content-Type:application/json Authorization:Bearer %s], Payload=%s", litellmURL+"/model/delete", obfuscatedKey, string(jsonPayload))
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", litellmURL+"/model/delete", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+litellmApiKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		if debugMode {
			log.Printf("Error removing model with ID %s: Status=%d, Body=%s", id, resp.StatusCode, string(body))
		}
		return fmt.Errorf("non-200 status: %s, body: %s", resp.Status, string(body))
	}
	if debugMode {
		log.Printf("Successfully removed model with ID %s: Status=200, Body=%s", id, string(body))
	}
	return nil
}

func main() {
	// Get sleep interval from environment, default to 60 seconds
	sleepIntervalStr := os.Getenv("SLEEP_INTERVAL")
	sleepInterval, err := strconv.Atoi(sleepIntervalStr)
	if err != nil {
		sleepInterval = 60
	}

	log.Println("Starting LiteLLM Configurator")
	for {
		// Read configuration
		config, err := readConfig("/etc/config/config.yaml")
		if err != nil {
			log.Println("Error reading config:", err)
			time.Sleep(time.Duration(sleepInterval) * time.Second)
			continue
		}

		// Read LiteLLM API key
		litellmApiKey, err := ioutil.ReadFile("/etc/secrets/litellm")
		if err != nil {
			log.Println("Error reading LiteLLM API key:", err)
			time.Sleep(time.Duration(sleepInterval) * time.Second)
			continue
		}

		// Get current model entries from LiteLLM
		currentModels, err := getCurrentModels(config.Litellm.URL, string(litellmApiKey))
		if err != nil {
			log.Println("Error getting current models from LiteLLM:", err)
			time.Sleep(time.Duration(sleepInterval) * time.Second)
			continue
		}

		// Build desired model entries from configured backends
		var desiredModels []DesiredModelEntry
		configuredBackends := make(map[string]bool)
		for _, backend := range config.Backends {
			configuredBackends[backend.URL] = true
			apiKeyPath := "/etc/secrets/" + backend.Name
			apiKey, err := ioutil.ReadFile(apiKeyPath)
			if err != nil {
				log.Printf("Error reading API key for'announced %s: %v", backend.Name, err)
				continue
			}

			models, err := getModels(backend.URL, string(apiKey))
			if err != nil {
				log.Printf("Error getting models from %s: %v", backend.Name, err)
				continue
			}

			for _, model := range models {
				entry := DesiredModelEntry{
					ModelName: model,
					LitellmParams: struct {
						Model   string `json:"model"`
						ApiBase string `json:"api_base"`
						ApiKey  string `json:"api_key"`
					}{
						Model:   "openai/" + model,
						ApiBase: backend.URL,
						ApiKey:  string(apiKey),
					},
				}
				desiredModels = append(desiredModels, entry)
			}
		}

		// Create sets for comparison
		currentSet := make(map[string]CurrentModelEntry)
		for _, entry := range currentModels {
			if configuredBackends[entry.LitellmParams.ApiBase] {
				key := fmt.Sprintf("%s|%s", entry.ModelName, entry.LitellmParams.ApiBase)
				currentSet[key] = entry
			}
		}

		desiredSet := make(map[string]DesiredModelEntry)
		for _, entry := range desiredModels {
			key := fmt.Sprintf("%s|%s", entry.ModelName, entry.LitellmParams.ApiBase)
			desiredSet[key] = entry
		}

		// Optional debug logging for model counts
		if debugMode {
			log.Printf("Current models from configured backends: %d", len(currentSet))
			log.Printf("Desired models: %d", len(desiredSet))
		}

		// Determine entries to add
		var toAdd []DesiredModelEntry
		for key, entry := range desiredSet {
			if _, exists := currentSet[key]; !exists {
				toAdd = append(toAdd, entry)
			}
		}

		// Determine entries to remove (only from configured backends)
		var toRemove []CurrentModelEntry
		for key, entry := range currentSet {
			if _, exists := desiredSet[key]; !exists {
				toRemove = append(toRemove, entry)
			}
		}

		// Add new models
		for _, entry := range toAdd {
			log.Printf("Adding model %s from %s", entry.ModelName, entry.LitellmParams.ApiBase)
			err := addModel(config.Litellm.URL, string(litellmApiKey), entry)
			if err != nil {
				log.Printf("Error adding model %s: %v", entry.ModelName, err)
			}
		}

		// Remove obsolete models (only from configured backends)
		for _, entry := range toRemove {
			log.Printf("Removing model %s from %s with ID %s", entry.ModelName, entry.LitellmParams.ApiBase, entry.ModelInfo.ID)
			err := removeModel(config.Litellm.URL, string(litellmApiKey), entry.ModelInfo.ID)
			if err != nil {
				log.Printf("Error removing model %s with ID %s: %v", entry.ModelName, entry.ModelInfo.ID, err)
			}
		}

		time.Sleep(time.Duration(sleepInterval) * time.Second)
	}
}
