package api

import (
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

type streamConfigResponse struct {
	ID         string `json:"id"`
	Protocol   string `json:"protocol"`
	Direction  string `json:"direction"`
	Duration   int    `json:"duration"`
	Streams    int    `json:"streams"`
	PacketSize int    `json:"packet_size"`
	ClientIP   string `json:"client_ip,omitempty"`
}

type streamSnapshotResponse struct {
	Config    streamConfigResponse `json:"config"`
	Status    string               `json:"status"`
	Progress  float64              `json:"progress"`
	Metrics   types.Metrics        `json:"metrics"`
	Network   *types.NetworkInfo   `json:"network,omitempty"`
	StartTime time.Time            `json:"start_time"`
	EndTime   time.Time            `json:"end_time"`
	Error     string               `json:"error,omitempty"`
}

func toStreamSnapshotResponse(snapshot types.StreamSnapshot) streamSnapshotResponse {
	resp := streamSnapshotResponse{
		Config: streamConfigResponse{
			ID:         snapshot.Config.ID,
			Protocol:   string(snapshot.Config.Protocol),
			Direction:  string(snapshot.Config.Direction),
			Duration:   int(snapshot.Config.Duration.Seconds()),
			Streams:    snapshot.Config.Streams,
			PacketSize: snapshot.Config.PacketSize,
			ClientIP:   snapshot.Config.ClientIP,
		},
		Status:    string(snapshot.Status),
		Progress:  snapshot.Progress,
		Metrics:   snapshot.Metrics,
		Network:   snapshot.Network,
		StartTime: snapshot.StartTime,
		EndTime:   snapshot.EndTime,
	}
	if snapshot.Error != nil {
		resp.Error = snapshot.Error.Error()
	}
	return resp
}
