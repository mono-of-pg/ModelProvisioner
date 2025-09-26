package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
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
		Name        string `yaml:"name"`
		URL         string `yaml:"url"`
		Discovery   bool   `yaml:"discovery"`
		FilterRegex string `yaml:"filter_regex"`
		Overrides   []struct {
			Regex        string                 `yaml:"regex"`
			Capabilities map[string]interface{} `yaml:"capabilities"`
		} `yaml:"overrides"`
		ModelInfoDefaults map[string]interface{} `yaml:"model_info_defaults"`
	} `yaml:"backends"`
}

// DesiredModelEntry represents a model entry to be added to LiteLLM
type DesiredModelEntry struct {
	ModelName     string `json:"model_name"`
	LitellmParams struct {
		Model   string `json:"model"`
		ApiBase string `json:"api_base"`
		ApiKey  string `json:"api_key"`
	} `json:"litellm_params"`
	ModelInfo map[string]interface{} `json:"model_info"`
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

// DeleteModelPayload represents the payload for deleting a model from LiteLLM
type DeleteModelPayload struct {
	ID string `json:"id"`
}

var debugMode bool

func init() {
	if os.Getenv("DEBUG") == "true" {
		debugMode = true
	}
}

func obfuscateKey(key string) string {
	if len(key) < 6 {
		return "REDACTED"
	}
	return key[:4] + "..REDACTED.." + key[len(key)-2:]
}

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

func testToolUse(backendURL, apiKey, model string) bool {
	client := &http.Client{Timeout: 10 * time.Second}
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": "What is the weather?"},
		},
		"tools": []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name": "get_weather",
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]string{"type": "string"},
						},
					},
				},
			},
		},
	}
	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", backendURL+"/chat/completions", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if message, ok := choice["message"].(map[string]interface{}); ok {
						if _, hasToolCalls := message["tool_calls"]; hasToolCalls {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func testVision(backendURL, apiKey, model string) bool {
	client := &http.Client{Timeout: 10 * time.Second}
	base64Image := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z/C/HgAGgwJ/lK3Q6wAAAABJRU5ErkJggg=="
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "text", "text": "Describe this image"},
					{"type": "image_url", "image_url": base64Image},
				},
			},
		},
	}
	jsonPayload, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", backendURL+"/chat/completions", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func applyOverrides(model string, overrides []struct {
	Regex        string                 `yaml:"regex"`
	Capabilities map[string]interface{} `yaml:"capabilities"`
}) map[string]interface{} {
	for _, override := range overrides {
		if matched, _ := regexp.MatchString(override.Regex, model); matched {
			return override.Capabilities
		}
	}
	return nil
}

func main() {
	sleepIntervalStr := os.Getenv("SLEEP_INTERVAL")
	sleepInterval, err := strconv.Atoi(sleepIntervalStr)
	if err != nil {
		sleepInterval = 60
	}

	log.Println("Starting LiteLLM ModelProvisioner (https://github.com/mono-of-pg/ModelProvisioner)")
	for {
		config, err := readConfig("/etc/config/config.yaml")
		if err != nil {
			log.Println("Error reading config:", err)
			time.Sleep(time.Duration(sleepInterval) * time.Second)
			continue
		}

		litellmApiKey, err := ioutil.ReadFile("/etc/secrets/litellm")
		if err != nil {
			log.Println("Error reading LiteLLM API key:", err)
			time.Sleep(time.Duration(sleepInterval) * time.Second)
			continue
		}

		currentModels, err := getCurrentModels(config.Litellm.URL, string(litellmApiKey))
		if err != nil {
			log.Println("Error getting current models from LiteLLM:", err)
			time.Sleep(time.Duration(sleepInterval) * time.Second)
			continue
		}

		var desiredModels []DesiredModelEntry
		configuredBackends := make(map[string]bool)
		for _, backend := range config.Backends {
			configuredBackends[backend.URL] = true
			apiKeyPath := "/etc/secrets/" + backend.Name
			apiKey, err := ioutil.ReadFile(apiKeyPath)
			if err != nil {
				if debugMode {
					log.Printf("API key not found for backend %s, using BLANK", backend.Name)
				}
				apiKey = []byte("BLANK")
			}

			var filterRegex *regexp.Regexp
			if backend.FilterRegex != "" {
				filterRegex, err = regexp.Compile(backend.FilterRegex)
				if err != nil {
					log.Printf("Invalid filter regex for backend %s: %v", backend.Name, err)
					continue
				}
			}

			models, err := getModels(backend.URL, string(apiKey))
			if err != nil {
				log.Printf("Error getting models from %s: %v", backend.Name, err)
				continue
			}

			for _, model := range models {
				if filterRegex != nil && !filterRegex.MatchString(model) {
					continue // Skip models that don't match the regex
				}
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
					ModelInfo: make(map[string]interface{}),
				}

				for k, v := range backend.ModelInfoDefaults {
					entry.ModelInfo[k] = v
				}

				if overrideCaps := applyOverrides(model, backend.Overrides); overrideCaps != nil {
					for k, v := range overrideCaps {
						entry.ModelInfo[k] = v
					}
				}

				desiredModels = append(desiredModels, entry)
			}
		}

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

		var toAdd []DesiredModelEntry
		for key, entry := range desiredSet {
			if _, exists := currentSet[key]; !exists {
				toAdd = append(toAdd, entry)
			}
		}

		var toRemove []CurrentModelEntry
		for key, entry := range currentSet {
			if _, exists := desiredSet[key]; !exists {
				toRemove = append(toRemove, entry)
			}
		}

		for _, entry := range toAdd {
			backendURL := entry.LitellmParams.ApiBase
			model := entry.ModelName
			apiKey := entry.LitellmParams.ApiKey

			var backendConfig *struct {
				Name        string `yaml:"name"`
				URL         string `yaml:"url"`
				Discovery   bool   `yaml:"discovery"`
				FilterRegex string `yaml:"filter_regex"`
				Overrides   []struct {
					Regex        string                 `yaml:"regex"`
					Capabilities map[string]interface{} `yaml:"capabilities"`
				} `yaml:"overrides"`
			}
			for _, b := range config.Backends {
				if b.URL == backendURL {
					backendConfig = &b
					break
				}
			}

			if backendConfig != nil && backendConfig.Discovery {
				if _, exists := entry.ModelInfo["supports_function_calling"]; !exists {
					entry.ModelInfo["supports_function_calling"] = testToolUse(backendURL, apiKey, model)
				}
				if _, exists := entry.ModelInfo["supports_vision"]; !exists {
					entry.ModelInfo["supports_vision"] = testVision(backendURL, apiKey, model)
				}
			}

			log.Printf("Adding model %s from %s", entry.ModelName, entry.LitellmParams.ApiBase)
			err := addModel(config.Litellm.URL, string(litellmApiKey), entry)
			if err != nil {
				log.Printf("Error adding model %s: %v", entry.ModelName, err)
			}
		}

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
