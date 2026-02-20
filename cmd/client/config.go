package client

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const clientAPIKeyEnv = "OPENBYTE_API_KEY"

type ServerConfig struct {
	URL    string `yaml:"url"`
	Name   string `yaml:"name"`
	APIKey string `yaml:"api_key,omitempty"`
}

type ConfigFile struct {
	DefaultServer string                  `yaml:"default_server,omitempty"`
	Servers       map[string]ServerConfig `yaml:"servers,omitempty"`

	ServerURL  string `yaml:"server_url,omitempty"`
	APIKey     string `yaml:"api_key,omitempty"`
	Protocol   string `yaml:"protocol,omitempty"`
	Direction  string `yaml:"direction,omitempty"`
	Duration   int    `yaml:"duration,omitempty"`
	Streams    int    `yaml:"streams,omitempty"`
	PacketSize int    `yaml:"packet_size,omitempty"`
	ChunkSize  int    `yaml:"chunk_size,omitempty"`
	Timeout    int    `yaml:"timeout,omitempty"`
	JSON       bool   `yaml:"json,omitempty"`
	Plain      bool   `yaml:"plain,omitempty"`
	Verbose    bool   `yaml:"verbose,omitempty"`
	Quiet      bool   `yaml:"quiet,omitempty"`
	NoColor    bool   `yaml:"no_color,omitempty"`
	NoProgress bool   `yaml:"no_progress,omitempty"`
}

func getConfigPath() string {
	configDir := resolvedConfigDir()
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, "openbyte", "config.yaml")
}

func getLegacyConfigPath() string {
	configDir := resolvedConfigDir()
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, "obyte", "config.yaml")
}

func resolvedConfigDir() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir != "" {
		return configDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config")
}

func loadConfigFile() (*ConfigFile, error) {
	configPath := getConfigPath()
	if configPath == "" {
		return nil, nil
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		legacyPath := getLegacyConfigPath()
		if legacyPath == "" {
			return nil, nil
		}
		if _, legacyErr := os.Stat(legacyPath); os.IsNotExist(legacyErr) {
			return nil, nil
		}
		configPath = legacyPath
		fmt.Fprintf(os.Stderr, "openbyte client: note: using legacy config path %s\n", legacyPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var config ConfigFile
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if err := validateConfigFile(&config); err != nil {
		return nil, fmt.Errorf("invalid config file: %w", err)
	}

	return &config, nil
}

func resolveServerURL(configFile *ConfigFile, serverAlias string) (string, string) {
	if configFile == nil {
		return "", ""
	}

	if serverAlias == "" {
		serverAlias = configFile.DefaultServer
	}

	if serverAlias != "" && configFile.Servers != nil {
		if server, ok := configFile.Servers[serverAlias]; ok {
			return server.URL, server.APIKey
		}
	}

	return configFile.ServerURL, configFile.APIKey
}

func mergeConfig(flagConfig *Config, configFile *ConfigFile, flagsSet map[string]bool) *Config {
	result := &Config{}

	applyDefaults(result)
	applyConfigFileDefaults(result, configFile, flagConfig.Server)

	if os.Getenv("NO_COLOR") != "" {
		result.NoColor = true
	}

	applyFlagOverrides(result, flagConfig, configFile, flagsSet)

	return result
}

func applyDefaults(result *Config) {
	result.ServerURL = defaultServerURL
	result.APIKey = strings.TrimSpace(os.Getenv(clientAPIKeyEnv))
	result.Protocol = defaultProtocol
	result.Direction = defaultDirection
	result.Duration = defaultDuration
	result.Streams = defaultStreams
	result.PacketSize = defaultPacketSize
	result.ChunkSize = defaultChunkSize
	result.Timeout = defaultTimeout
	result.WarmUp = defaultWarmUp
}

func applyConfigFileDefaults(result *Config, configFile *ConfigFile, flagServer string) {
	if configFile == nil {
		return
	}
	serverURL, apiKey := resolveServerURL(configFile, flagServer)
	if serverURL != "" {
		result.ServerURL = serverURL
	} else if configFile.ServerURL != "" {
		result.ServerURL = configFile.ServerURL
	}
	if apiKey != "" {
		result.APIKey = apiKey
	} else if configFile.APIKey != "" {
		result.APIKey = configFile.APIKey
	}
	if configFile.Protocol != "" {
		result.Protocol = configFile.Protocol
	}
	if configFile.Direction != "" {
		result.Direction = configFile.Direction
	}
	if configFile.Duration > 0 {
		result.Duration = configFile.Duration
	}
	if configFile.Streams > 0 {
		result.Streams = configFile.Streams
	}
	if configFile.PacketSize > 0 {
		result.PacketSize = configFile.PacketSize
	}
	if configFile.ChunkSize > 0 {
		result.ChunkSize = configFile.ChunkSize
	}
	if configFile.Timeout > 0 {
		result.Timeout = configFile.Timeout
	}
	result.JSON = configFile.JSON
	result.Plain = configFile.Plain
	result.Verbose = configFile.Verbose
	result.Quiet = configFile.Quiet
	result.NoColor = configFile.NoColor
	result.NoProgress = configFile.NoProgress
}

func applyFlagOverrides(result, flagConfig *Config, configFile *ConfigFile, flagsSet map[string]bool) {
	if flagsSet["server"] && flagConfig.Server != "" {
		applyServerFlagOverride(result, flagConfig, configFile)
	}
	applyStringOverride(flagsSet, "server-url", flagConfig.ServerURL, func(v string) { result.ServerURL = v })
	applyStringOverride(flagsSet, "protocol", flagConfig.Protocol, func(v string) { result.Protocol = v })
	applyStringOverride(flagsSet, "direction", flagConfig.Direction, func(v string) { result.Direction = v })
	applyPositiveIntOverride(flagsSet, "duration", flagConfig.Duration, func(v int) { result.Duration = v })
	applyPositiveIntOverride(flagsSet, "streams", flagConfig.Streams, func(v int) { result.Streams = v })
	applyPositiveIntOverride(flagsSet, "packet-size", flagConfig.PacketSize, func(v int) { result.PacketSize = v })
	applyPositiveIntOverride(flagsSet, "chunk-size", flagConfig.ChunkSize, func(v int) { result.ChunkSize = v })
	applyPositiveIntOverride(flagsSet, "timeout", flagConfig.Timeout, func(v int) { result.Timeout = v })
	applyBoolOverride(flagsSet, "json", flagConfig.JSON, func(v bool) { result.JSON = v })
	applyBoolOverride(flagsSet, "plain", flagConfig.Plain, func(v bool) { result.Plain = v })
	applyBoolOverride(flagsSet, "verbose", flagConfig.Verbose, func(v bool) { result.Verbose = v })
	applyBoolOverride(flagsSet, "quiet", flagConfig.Quiet, func(v bool) { result.Quiet = v })
	applyBoolOverride(flagsSet, "no-color", flagConfig.NoColor, func(v bool) { result.NoColor = v })
	applyBoolOverride(flagsSet, "no-progress", flagConfig.NoProgress, func(v bool) { result.NoProgress = v })
	applyIntOverride(flagsSet, "warmup", flagConfig.WarmUp, func(v int) { result.WarmUp = v })
	applyBoolOverride(flagsSet, "auto", flagConfig.Auto, func(v bool) { result.Auto = v })
}

func applyStringOverride(flagsSet map[string]bool, key string, value string, apply func(string)) {
	if !flagsSet[key] || value == "" {
		return
	}
	apply(value)
}

func applyPositiveIntOverride(flagsSet map[string]bool, key string, value int, apply func(int)) {
	if !flagsSet[key] || value <= 0 {
		return
	}
	apply(value)
}

func applyBoolOverride(flagsSet map[string]bool, key string, value bool, apply func(bool)) {
	if !flagsSet[key] {
		return
	}
	apply(value)
}

func applyIntOverride(flagsSet map[string]bool, key string, value int, apply func(int)) {
	if !flagsSet[key] {
		return
	}
	apply(value)
}

func applyServerFlagOverride(result, flagConfig *Config, configFile *ConfigFile) {
	if configFile == nil || configFile.Servers == nil {
		result.ServerURL = flagConfig.Server
		return
	}
	server, ok := configFile.Servers[flagConfig.Server]
	if !ok {
		result.ServerURL = flagConfig.Server
		return
	}
	result.ServerURL = server.URL
	if server.APIKey != "" {
		result.APIKey = server.APIKey
	}
}

func validateConfigFile(config *ConfigFile) error {
	if err := validateServerURLs(config); err != nil {
		return err
	}
	if err := validateProtocolAndDirection(config); err != nil {
		return err
	}
	return validateNumericRanges(config)
}

func validateServerURLs(config *ConfigFile) error {
	if config.ServerURL != "" {
		if _, err := normalizeAndValidateServerURL(config.ServerURL); err != nil {
			return fmt.Errorf("invalid server_url: %w", err)
		}
	}
	for alias, server := range config.Servers {
		if strings.TrimSpace(server.URL) == "" {
			return fmt.Errorf("invalid server %q url: empty", alias)
		}
		if _, err := normalizeAndValidateServerURL(server.URL); err != nil {
			return fmt.Errorf("invalid server %q url: %w", alias, err)
		}
	}
	return nil
}

func validateProtocolAndDirection(config *ConfigFile) error {
	if config.Protocol != "" && config.Protocol != "tcp" && config.Protocol != "udp" && config.Protocol != "http" {
		return fmt.Errorf("invalid protocol: %s (must be tcp, udp, or http)", config.Protocol)
	}
	if config.Direction != "" && config.Direction != "download" && config.Direction != "upload" && config.Direction != "bidirectional" {
		return fmt.Errorf("invalid direction: %s (must be download, upload, or bidirectional)", config.Direction)
	}
	if config.Protocol == "http" && config.Direction == "bidirectional" {
		return fmt.Errorf("invalid direction for http: %s (must be download or upload)", config.Direction)
	}
	return nil
}

func validateNumericRanges(config *ConfigFile) error {
	if config.Duration != 0 && (config.Duration < 1 || config.Duration > 300) {
		return fmt.Errorf("invalid duration: %d (must be 1-300 seconds)", config.Duration)
	}
	if config.Streams != 0 && (config.Streams < 1 || config.Streams > 64) {
		return fmt.Errorf("invalid streams: %d (must be 1-64)", config.Streams)
	}
	if config.PacketSize < 0 || (config.PacketSize > 0 && (config.PacketSize < 64 || config.PacketSize > 9000)) {
		return fmt.Errorf("invalid packet size: %d (must be 64-9000 bytes)", config.PacketSize)
	}
	if config.ChunkSize < 0 || (config.ChunkSize > 0 && (config.ChunkSize < 65536 || config.ChunkSize > 4194304)) {
		return fmt.Errorf("invalid chunk size: %d (must be 65536-4194304 bytes)", config.ChunkSize)
	}
	if config.Timeout < 0 {
		return fmt.Errorf("invalid timeout: %d (must be positive)", config.Timeout)
	}
	return nil
}

func normalizeAndValidateServerURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("host is required")
	}
	if u.RawQuery != "" {
		return "", fmt.Errorf("query is not allowed")
	}
	if u.Fragment != "" {
		return "", fmt.Errorf("fragment is not allowed")
	}
	if port := u.Port(); port != "" {
		n, convErr := strconv.Atoi(port)
		if convErr != nil || n < 1 || n > 65535 {
			return "", fmt.Errorf("port must be in range 1-65535")
		}
	}
	return strings.TrimRight(u.String(), "/"), nil
}
