package client

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

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
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "openbyte", "config.yaml")
}

func getLegacyConfigPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "obyte", "config.yaml")
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

	result.ServerURL = defaultServerURL
	result.Protocol = defaultProtocol
	result.Direction = defaultDirection
	result.Duration = defaultDuration
	result.Streams = defaultStreams
	result.PacketSize = defaultPacketSize
	result.ChunkSize = defaultChunkSize
	result.Timeout = defaultTimeout

	if configFile != nil {
		serverURL, apiKey := resolveServerURL(configFile, flagConfig.Server)
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

	if val := os.Getenv("OBYTE_SERVER_URL"); val != "" {
		result.ServerURL = val
	}
	if val := os.Getenv("OBYTE_API_KEY"); val != "" {
		result.APIKey = val
	}
	if val := os.Getenv("OBYTE_PROTOCOL"); val != "" {
		result.Protocol = val
	}
	if val := os.Getenv("OBYTE_DIRECTION"); val != "" {
		result.Direction = val
	}
	if val := os.Getenv("OBYTE_DURATION"); val != "" {
		if d, err := strconv.Atoi(val); err == nil {
			result.Duration = d
		} else {
			fmt.Fprintf(os.Stderr, "openbyte client: warning: invalid OBYTE_DURATION value '%s' (must be integer), ignoring\n", val)
		}
	}
	if val := os.Getenv("OBYTE_STREAMS"); val != "" {
		if s, err := strconv.Atoi(val); err == nil {
			result.Streams = s
		} else {
			fmt.Fprintf(os.Stderr, "openbyte client: warning: invalid OBYTE_STREAMS value '%s' (must be integer), ignoring\n", val)
		}
	}
	if val := os.Getenv("OBYTE_PACKET_SIZE"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			result.PacketSize = p
		} else {
			fmt.Fprintf(os.Stderr, "openbyte client: warning: invalid OBYTE_PACKET_SIZE value '%s' (must be integer), ignoring\n", val)
		}
	}
	if val := os.Getenv("OBYTE_CHUNK_SIZE"); val != "" {
		if c, err := strconv.Atoi(val); err == nil {
			result.ChunkSize = c
		} else {
			fmt.Fprintf(os.Stderr, "openbyte client: warning: invalid OBYTE_CHUNK_SIZE value '%s' (must be integer), ignoring\n", val)
		}
	}
	if val := os.Getenv("OBYTE_TIMEOUT"); val != "" {
		if t, err := strconv.Atoi(val); err == nil {
			result.Timeout = t
		} else {
			fmt.Fprintf(os.Stderr, "openbyte client: warning: invalid OBYTE_TIMEOUT value '%s' (must be integer), ignoring\n", val)
		}
	}
	if val := os.Getenv("OBYTE_WARMUP"); val != "" {
		if w, err := strconv.Atoi(val); err == nil {
			result.WarmUp = w
		} else {
			fmt.Fprintf(os.Stderr, "openbyte client: warning: invalid OBYTE_WARMUP value '%s' (must be integer), ignoring\n", val)
		}
	}
	if os.Getenv("NO_COLOR") != "" {
		result.NoColor = true
	}

	if flagsSet["server"] && flagConfig.Server != "" {
		if configFile != nil && configFile.Servers != nil {
			if server, ok := configFile.Servers[flagConfig.Server]; ok {
				result.ServerURL = server.URL
				if server.APIKey != "" {
					result.APIKey = server.APIKey
				}
			} else {
				result.ServerURL = flagConfig.Server
			}
		} else {
			result.ServerURL = flagConfig.Server
		}
	}
	if flagsSet["server-url"] && flagConfig.ServerURL != "" {
		result.ServerURL = flagConfig.ServerURL
	}
	if flagsSet["protocol"] && flagConfig.Protocol != "" {
		result.Protocol = flagConfig.Protocol
	}
	if flagsSet["direction"] && flagConfig.Direction != "" {
		result.Direction = flagConfig.Direction
	}
	if flagsSet["duration"] && flagConfig.Duration > 0 {
		result.Duration = flagConfig.Duration
	}
	if flagsSet["streams"] && flagConfig.Streams > 0 {
		result.Streams = flagConfig.Streams
	}
	if flagsSet["packet-size"] && flagConfig.PacketSize > 0 {
		result.PacketSize = flagConfig.PacketSize
	}
	if flagsSet["chunk-size"] && flagConfig.ChunkSize > 0 {
		result.ChunkSize = flagConfig.ChunkSize
	}
	if flagsSet["timeout"] && flagConfig.Timeout > 0 {
		result.Timeout = flagConfig.Timeout
	}
	if flagsSet["json"] {
		result.JSON = flagConfig.JSON
	}
	if flagsSet["plain"] {
		result.Plain = flagConfig.Plain
	}
	if flagsSet["verbose"] {
		result.Verbose = flagConfig.Verbose
	}
	if flagsSet["quiet"] {
		result.Quiet = flagConfig.Quiet
	}
	if flagsSet["no-color"] {
		result.NoColor = flagConfig.NoColor
	}
	if flagsSet["no-progress"] {
		result.NoProgress = flagConfig.NoProgress
	}
	if flagsSet["api-key"] && flagConfig.APIKey != "" {
		result.APIKey = flagConfig.APIKey
	}

	if flagsSet["warmup"] {
		result.WarmUp = flagConfig.WarmUp
	}
	if flagsSet["auto"] {
		result.Auto = flagConfig.Auto
	}

	return result
}

func validateConfigFile(config *ConfigFile) error {
	if config.Protocol != "" && config.Protocol != "tcp" && config.Protocol != "udp" && config.Protocol != "http" {
		return fmt.Errorf("invalid protocol: %s (must be tcp, udp, or http)", config.Protocol)
	}
	if config.Direction != "" && config.Direction != "download" && config.Direction != "upload" && config.Direction != "bidirectional" {
		return fmt.Errorf("invalid direction: %s (must be download, upload, or bidirectional)", config.Direction)
	}
	if config.Protocol == "http" && config.Direction == "bidirectional" {
		return fmt.Errorf("invalid direction for http: %s (must be download or upload)", config.Direction)
	}
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
