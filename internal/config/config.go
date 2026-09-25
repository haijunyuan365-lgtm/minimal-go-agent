package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

const DefaultPath = "config.yml"

var envReference = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)

type Config struct {
	Server ServerConfig `yaml:"server"`
	LLM    LLMConfig    `yaml:"llm"`
	Agent  AgentConfig  `yaml:"agent"`
}

type ServerConfig struct {
	ListenAddr               string `yaml:"listen_addr"`
	DatabasePath             string `yaml:"database_path"`
	ReadHeaderTimeoutSeconds int    `yaml:"read_header_timeout_seconds"`
	ReadTimeoutSeconds       int    `yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds      int    `yaml:"write_timeout_seconds"`
	IdleTimeoutSeconds       int    `yaml:"idle_timeout_seconds"`
	ShutdownTimeoutSeconds   int    `yaml:"shutdown_timeout_seconds"`
}

type LLMConfig struct {
	Provider string         `yaml:"provider"`
	OpenAI   ProviderConfig `yaml:"openai"`
	DeepSeek ProviderConfig `yaml:"deepseek"`
}

type ProviderConfig struct {
	APIKey                string `yaml:"api_key"`
	Model                 string `yaml:"model"`
	Endpoint              string `yaml:"endpoint"`
	RequestTimeoutSeconds int    `yaml:"request_timeout_seconds"`
	ReasoningSummary      string `yaml:"reasoning_summary"`
	ReasoningEffort       string `yaml:"reasoning_effort"`
}

type AgentConfig struct {
	MaxLLMCalls        int `yaml:"max_llm_calls"`
	MaxToolCalls       int `yaml:"max_tool_calls"`
	MaxMessageChars    int `yaml:"max_message_chars"`
	MaxRecentTurns     int `yaml:"max_recent_turns"`
	ContextCharLimit   int `yaml:"context_char_limit"`
	RecentCharBudget   int `yaml:"recent_char_budget"`
	MaxSummaryChars    int `yaml:"max_summary_chars"`
	MaxCompactionCalls int `yaml:"max_compaction_calls"`
}

// Load reads config.yml, or the file named by AGENT_CONFIG.
func Load() (Config, error) {
	path := strings.TrimSpace(os.Getenv("AGENT_CONFIG"))
	if path == "" {
		path = DefaultPath
	}
	return LoadFile(path)
}

func LoadFile(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return Config{}, errors.New("config path must not be empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	if len(data) > 1<<20 {
		return Config{}, errors.New("config file exceeds 1 MiB")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, errors.New("config must contain exactly one YAML document")
		}
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config %q: %w", path, err)
	}
	return cfg, nil
}

func (cfg Config) SelectedLLM() ProviderConfig {
	if cfg.LLM.Provider == "deepseek" {
		return cfg.LLM.DeepSeek
	}
	return cfg.LLM.OpenAI
}

// ResolveAPIKey expands a whole-value ${NAME} reference, or returns a literal key.
func ResolveAPIKey(value string) string {
	value = strings.TrimSpace(value)
	if match := envReference.FindStringSubmatch(value); match != nil {
		return strings.TrimSpace(os.Getenv(match[1]))
	}
	return value
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.Server.ListenAddr) == "" || strings.TrimSpace(cfg.Server.DatabasePath) == "" {
		return errors.New("server.listen_addr and server.database_path are required")
	}
	if cfg.Server.ReadHeaderTimeoutSeconds < 1 || cfg.Server.ReadTimeoutSeconds < 1 || cfg.Server.WriteTimeoutSeconds < 1 ||
		cfg.Server.IdleTimeoutSeconds < 1 || cfg.Server.ShutdownTimeoutSeconds < 1 {
		return errors.New("all server timeouts must be positive")
	}
	if cfg.LLM.Provider != "openai" && cfg.LLM.Provider != "deepseek" {
		return errors.New("llm.provider must be openai or deepseek")
	}
	selected := cfg.SelectedLLM()
	if strings.TrimSpace(selected.Model) == "" {
		return errors.New("selected llm model is required")
	}
	if selected.RequestTimeoutSeconds < 1 {
		return errors.New("selected llm request_timeout_seconds must be positive")
	}
	if err := validateEndpoint(selected.Endpoint); err != nil {
		return err
	}
	if cfg.LLM.Provider == "openai" {
		if selected.ReasoningSummary != "" && selected.ReasoningSummary != "auto" {
			return errors.New("llm.openai.reasoning_summary must be empty or auto")
		}
	} else if selected.ReasoningEffort != "" && selected.ReasoningEffort != "none" && selected.ReasoningEffort != "low" && selected.ReasoningEffort != "high" && selected.ReasoningEffort != "max" {
		return errors.New("llm.deepseek.reasoning_effort must be none, low, high, or max")
	}
	limits := cfg.Agent
	if limits.MaxLLMCalls < 1 || limits.MaxToolCalls < 1 || limits.MaxMessageChars < 1 || limits.MaxRecentTurns < 1 ||
		limits.ContextCharLimit < 1 || limits.RecentCharBudget < 1 ||
		limits.MaxSummaryChars < 1 || limits.MaxCompactionCalls < 1 {
		return errors.New("all agent limits must be positive")
	}
	return nil
}

func validateEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		!strings.HasSuffix(parsed.Path, "/responses") {
		return errors.New("selected llm endpoint must be an absolute HTTP(S) URL ending in /responses")
	}
	return nil
}
