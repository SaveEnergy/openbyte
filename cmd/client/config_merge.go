package client

import (
	"os"
)

func mergeConfig(flagConfig *Config, configFile *ConfigFile, flagsSet map[string]bool) *Config {
	result := &Config{}

	applyDefaults(result)
	applyConfigFileDefaults(result, configFile)

	if os.Getenv("NO_COLOR") != "" {
		result.NoColor = true
	}

	applyFlagOverrides(result, flagConfig, flagsSet)

	return result
}

func applyDefaults(result *Config) {
	result.ServerURL = defaultServerURL
	result.Direction = defaultDirection
	result.Duration = defaultDuration
	result.Streams = defaultStreams
	result.ChunkSize = defaultChunkSize
	result.Timeout = defaultTimeout
	result.WarmUp = defaultWarmUp
}

func applyConfigFileDefaults(result *Config, configFile *ConfigFile) {
	if configFile == nil {
		return
	}
	if configFile.ServerURL != "" {
		result.ServerURL = configFile.ServerURL
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

func applyFlagOverrides(result, flagConfig *Config, flagsSet map[string]bool) {
	applyStringOverride(flagsSet, "server-url", flagConfig.ServerURL, func(v string) { result.ServerURL = v })
	applyStringOverride(flagsSet, "direction", flagConfig.Direction, func(v string) { result.Direction = v })
	applyPositiveIntOverride(flagsSet, "duration", flagConfig.Duration, func(v int) { result.Duration = v })
	applyPositiveIntOverride(flagsSet, "streams", flagConfig.Streams, func(v int) { result.Streams = v })
	applyPositiveIntOverride(flagsSet, "chunk-size", flagConfig.ChunkSize, func(v int) { result.ChunkSize = v })
	applyPositiveIntOverride(flagsSet, "timeout", flagConfig.Timeout, func(v int) { result.Timeout = v })
	applyBoolOverride(flagsSet, "json", flagConfig.JSON, func(v bool) { result.JSON = v })
	applyBoolOverride(flagsSet, "plain", flagConfig.Plain, func(v bool) { result.Plain = v })
	applyBoolOverride(flagsSet, "verbose", flagConfig.Verbose, func(v bool) { result.Verbose = v })
	applyBoolOverride(flagsSet, "quiet", flagConfig.Quiet, func(v bool) { result.Quiet = v })
	applyBoolOverride(flagsSet, "no-color", flagConfig.NoColor, func(v bool) { result.NoColor = v })
	applyBoolOverride(flagsSet, "no-progress", flagConfig.NoProgress, func(v bool) { result.NoProgress = v })
	applyIntOverride(flagsSet, "warmup", flagConfig.WarmUp, func(v int) { result.WarmUp = v })
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
