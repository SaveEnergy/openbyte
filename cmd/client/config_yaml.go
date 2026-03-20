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
	if config.Protocol != "" && config.Protocol != protocolTCP && config.Protocol != protocolUDP && config.Protocol != protocolHTTP {
		return fmt.Errorf("invalid protocol: %s (must be tcp, udp, or http)", config.Protocol)
	}
	if config.Direction != "" && config.Direction != directionDownload && config.Direction != directionUpload && config.Direction != directionBidirectional {
		return fmt.Errorf("invalid direction: %s (must be download, upload, or bidirectional)", config.Direction)
	}
	if config.Protocol == protocolHTTP && config.Direction == directionBidirectional {
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
	if u.Scheme != schemeHTTP && u.Scheme != schemeHTTPS {
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
