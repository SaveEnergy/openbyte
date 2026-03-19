package client

import "fmt"

func validateConfig(config *Config) error {
	if err := validateProtocol(config); err != nil {
		return err
	}
	if err := validateDirection(config); err != nil {
		return err
	}
	if err := validateNumericConfig(config); err != nil {
		return err
	}
	normalizedURL, err := normalizeAndValidateServerURL(config.ServerURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	config.ServerURL = normalizedURL
	return nil
}

func validateProtocol(config *Config) error {
	if config.Protocol != protocolTCP && config.Protocol != protocolUDP && config.Protocol != protocolHTTP {
		return fmt.Errorf("invalid protocol: %s\n\n"+
			"Protocol must be 'tcp', 'udp', or 'http'.\n"+
			"Use: openbyte client -p tcp  or  openbyte client -p udp  or  openbyte client -p http\n"+
			helpHintSuffix, config.Protocol)
	}
	if config.Protocol == protocolHTTP && config.Direction == directionBidirectional {
		return fmt.Errorf("invalid direction for http: %s\n\n"+
			"HTTP protocol supports 'download' or 'upload'.\n"+
			"Use: openbyte client -p http -d download  or  openbyte client -p http -d upload\n"+
			helpHintSuffix, config.Direction)
	}
	return nil
}

func validateDirection(config *Config) error {
	if config.Direction != directionDownload && config.Direction != directionUpload && config.Direction != directionBidirectional {
		return fmt.Errorf("invalid direction: %s\n\n"+
			"Direction must be 'download', 'upload', or 'bidirectional'.\n"+
			"Use: openbyte client -d download  or  openbyte client -d upload  or  openbyte client -d bidirectional\n"+
			helpHintSuffix, config.Direction)
	}
	return nil
}

func validateNumericConfig(config *Config) error {
	if config.Duration < 1 || config.Duration > 300 {
		return fmt.Errorf("invalid duration: %d\n\n"+
			"Duration must be between 1 and 300 seconds.\n"+
			"Use: openbyte client -t 30  (for 30 seconds)\n"+
			helpHintSuffix, config.Duration)
	}
	if config.Streams < 1 || config.Streams > 64 {
		return fmt.Errorf("invalid streams: %d\n\n"+
			"Streams must be between 1 and 64.\n"+
			"Use: openbyte client -s 4  (for 4 parallel streams)\n"+
			helpHintSuffix, config.Streams)
	}
	if config.Protocol != protocolHTTP {
		if config.PacketSize < 64 || config.PacketSize > 9000 {
			return fmt.Errorf("invalid packet size: %d\n\n"+
				"Packet size must be between 64 and 9000 bytes.\n"+
				"Use: openbyte client --packet-size 1400  (WAN-safe default)\n"+
				helpHintSuffix, config.PacketSize)
		}
	}
	if config.Protocol == protocolHTTP {
		if config.ChunkSize < 65536 || config.ChunkSize > 4194304 {
			return fmt.Errorf("invalid chunk size: %d\n\n"+
				"Chunk size must be between 65536 and 4194304 bytes.\n"+
				"Use: openbyte client --chunk-size 1048576  (1MB)\n"+
				helpHintSuffix, config.ChunkSize)
		}
	}
	return nil
}
