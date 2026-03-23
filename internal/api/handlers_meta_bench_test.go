package api

import (
	"encoding/json"
	"testing"

	"github.com/saveenergy/openbyte/pkg/types"
)

// BenchmarkMarshalServersResponse matches GET /api/v1/servers JSON payload shape.
func BenchmarkMarshalServersResponse(b *testing.B) {
	resp := ServersResponse{
		Servers: []types.ServerInfo{
			{
				ID:           "bench-srv",
				Name:         "Bench East",
				Location:     "NYC",
				Region:       "us-east",
				Host:         "speed.example.com",
				TCPPort:      8081,
				UDPPort:      8082,
				APIEndpoint:  "https://speed.example.com:8443",
				Health:       "healthy",
				CapacityGbps: 25,
				ActiveTests:  3,
				MaxTests:     10,
			},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, err := json.Marshal(resp)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMarshalVersionResponse matches GET /api/v1/version payload.
func BenchmarkMarshalVersionResponse(b *testing.B) {
	v := VersionResponse{Version: "0.0.0+bench"}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, err := json.Marshal(v)
		if err != nil {
			b.Fatal(err)
		}
	}
}
