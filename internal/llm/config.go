// Copyright © 2026 ソニーレベル <C7kali3@gmail.com>
// Configuration loading with precedence: CLI > ENV > config file > defaults

package llm

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the LLM configuration from file
type Config struct {
	Provider string            `json:"provider" yaml:"provider"`
	Model    string            `json:"model" yaml:"model"`
	Endpoint string            `json:"endpoint" yaml:"endpoint"`
	Token    string            `json:"token" yaml:"token"`
	Timeout  string            `json:"timeout" yaml:"timeout"` // e.g., "60s"
	Keys     map[string]string `json:"keys" yaml:"keys"`       // provider-specific keys
}

// ConfigPaths returns the paths to check for config files in order
func ConfigPaths() []string {
	var paths []string

	// Current directory
	paths = append(paths, ".readme-runner.yaml", ".readme-runner.yml", ".readme-runner.json")

	// XDG config directory
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths,
			filepath.Join(xdg, "readme-runner", "config.yaml"),
			filepath.Join(xdg, "readme-runner", "config.yml"),
			filepath.Join(xdg, "readme-runner", "config.json"),
		)
	}

	// Home directory
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".config", "readme-runner", "config.yaml"),
			filepath.Join(home, ".config", "readme-runner", "config.yml"),
			filepath.Join(home, ".config", "readme-runner", "config.json"),
			filepath.Join(home, ".readme-runner.yaml"),
			filepath.Join(home, ".readme-runner.yml"),
			filepath.Join(home, ".readme-runner.json"),
		)
	}

	return paths
}

// LoadConfig loads configuration from file
func LoadConfig() (*Config, error) {
	for _, path := range ConfigPaths() {
		cfg, err := loadConfigFromPath(path)
		if err == nil {
			return cfg, nil
		}
		// Continue if file not found, return error for parse failures
		if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no such file") {
			return nil, err
		}
	}
	return nil, nil // No config file found, not an error
}

func loadConfigFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	default:
		// Try YAML first, then JSON
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			if err := json.Unmarshal(data, &cfg); err != nil {
				return nil, err
			}
		}
	}

	return &cfg, nil
}

// ResolveProviderConfig creates a ProviderConfig with proper precedence:
// CLI flags > Environment variables > Config file > Defaults
func ResolveProviderConfig(cliProvider, cliEndpoint, cliModel, cliToken string, cliTimeout time.Duration, verbose bool) *ProviderConfig {
	// Load config file (optional)
	fileCfg, _ := LoadConfig()

	// Start with defaults
	config := &ProviderConfig{
		Type:    ProviderMock,
		Timeout: DefaultTimeout,
		Verbose: verbose,
	}

	// Apply config file values
	if fileCfg != nil {
		if fileCfg.Provider != "" {
			config.Type = ProviderType(fileCfg.Provider)
		}
		if fileCfg.Model != "" {
			config.Model = fileCfg.Model
		}
		if fileCfg.Endpoint != "" {
			config.Endpoint = fileCfg.Endpoint
		}
		if fileCfg.Token != "" {
			config.Token = fileCfg.Token
		}
		if fileCfg.Timeout != "" {
			if d, err := time.ParseDuration(fileCfg.Timeout); err == nil {
				config.Timeout = d
			}
		}
	}

	// Apply environment variables
	if envProvider := os.Getenv("RD_LLM_PROVIDER"); envProvider != "" {
		config.Type = ProviderType(envProvider)
	}
	if envModel := os.Getenv("RD_LLM_MODEL"); envModel != "" {
		config.Model = envModel
	}
	if envEndpoint := os.Getenv("RD_LLM_ENDPOINT"); envEndpoint != "" {
		config.Endpoint = envEndpoint
	}
	if envToken := os.Getenv("RD_LLM_TOKEN"); envToken != "" {
		config.Token = envToken
	}
	if envTimeout := os.Getenv("RD_LLM_TIMEOUT"); envTimeout != "" {
		if d, err := time.ParseDuration(envTimeout); err == nil {
			config.Timeout = d
		}
	}

	// Apply CLI flags (highest priority)
	if cliProvider != "" {
		config.Type = ProviderType(cliProvider)
	}
	if cliModel != "" {
		config.Model = cliModel
	}
	if cliEndpoint != "" {
		config.Endpoint = cliEndpoint
	}
	if cliToken != "" {
		config.Token = cliToken
	}
	if cliTimeout > 0 {
		config.Timeout = cliTimeout
	}

	// Auto-select provider if still mock and no explicit selection
	if config.Type == ProviderMock && cliProvider == "" && os.Getenv("RD_LLM_PROVIDER") == "" {
		config.Type = autoSelectProvider(config)
	}

	return config.WithDefaults()
}

// autoSelectProvider chooses the best available provider
// Priority: anthropic > openai > mistral > ollama > mock
func autoSelectProvider(config *ProviderConfig) ProviderType {
	// Check for Anthropic key
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return ProviderAnthropic
	}

	// Check for OpenAI key
	if os.Getenv("OPENAI_API_KEY") != "" {
		return ProviderOpenAI
	}

	// Check for Mistral key
	if os.Getenv("MISTRAL_API_KEY") != "" {
		return ProviderMistral
	}

	// Check if Ollama is running
	if IsOllamaAvailable() {
		return ProviderOllama
	}

	// Default to mock (offline mode)
	return ProviderMock
}

// GetProviderToken returns the appropriate token for a provider type
func GetProviderToken(providerType ProviderType, configToken string) string {
	// CLI/config token has highest priority
	if configToken != "" {
		return configToken
	}

	// Provider-specific environment variables
	switch providerType {
	case ProviderOpenAI:
		return os.Getenv("OPENAI_API_KEY")
	case ProviderAnthropic:
		return os.Getenv("ANTHROPIC_API_KEY")
	case ProviderMistral:
		return os.Getenv("MISTRAL_API_KEY")
	case ProviderOllama:
		return "" // No token needed
	case ProviderHTTP:
		return os.Getenv("RD_LLM_TOKEN")
	default:
		return os.Getenv("RD_LLM_TOKEN")
	}
}

// IsOllamaAvailable checks if Ollama is running locally
func IsOllamaAvailable() bool {
	endpoint := "http://localhost:11434/api/tags"
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		if !strings.HasPrefix(host, "http") {
			host = "http://" + host
		}
		endpoint = host + "/api/tags"
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
