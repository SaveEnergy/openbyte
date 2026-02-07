package types

// ServerInfo describes a speed test server instance, used by both the
// API handlers and the registry subsystem.
type ServerInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Location     string `json:"location"`
	Region       string `json:"region,omitempty"`
	Host         string `json:"host"`
	TCPPort      int    `json:"tcp_port"`
	UDPPort      int    `json:"udp_port"`
	APIEndpoint  string `json:"api_endpoint"`
	Health       string `json:"health"`
	CapacityGbps int    `json:"capacity_gbps"`
	ActiveTests  int    `json:"active_tests"`
	MaxTests     int    `json:"max_tests"`
}
