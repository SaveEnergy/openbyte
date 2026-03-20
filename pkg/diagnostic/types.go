// Package diagnostic interprets raw speed test metrics into human/agent-readable
// grades, ratings, and suitability assessments.
package diagnostic

// Interpretation holds the semantic interpretation of speed test results.
type Interpretation struct {
	Grade           string   `json:"grade"`
	Summary         string   `json:"summary"`
	LatencyRating   string   `json:"latency_rating"`
	SpeedRating     string   `json:"speed_rating"`
	StabilityRating string   `json:"stability_rating"`
	SuitableFor     []string `json:"suitable_for"`
	Concerns        []string `json:"concerns"`
}

// Params are the raw metrics to interpret.
type Params struct {
	DownloadMbps float64
	UploadMbps   float64
	LatencyMs    float64
	JitterMs     float64
	PacketLoss   float64
}
