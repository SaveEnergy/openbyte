package client

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/saveenergy/openbyte/pkg/types"
)

func (f *NDJSONFormatter) FormatProgress(progress, elapsed, remaining float64) {
	msg := map[string]any{
		"type":        "progress",
		"percent":     progress,
		"elapsed_s":   elapsed,
		"remaining_s": remaining,
	}
	f.encode(msg)
}

func (f *NDJSONFormatter) FormatMetrics(metrics *types.Metrics) {
	msg := map[string]any{
		"type":            "metrics",
		"throughput_mbps": metrics.ThroughputMbps,
		"bytes":           metrics.BytesTransferred,
		"latency_avg_ms":  metrics.Latency.AvgMs,
		"jitter_ms":       metrics.JitterMs,
	}
	f.encode(msg)
}

func (f *NDJSONFormatter) FormatComplete(results *StreamResults) {
	msg := map[string]any{
		"type": "result",
		"data": results,
	}
	f.encode(msg)
}

func (f *NDJSONFormatter) FormatError(err error) {
	msg := JSONErrorResponse{
		SchemaVersion: SchemaVersion,
		Error:         true,
		Code:          classifyErrorCode(err),
		Message:       err.Error(),
	}
	f.encode(msg)
}

func (f *NDJSONFormatter) LastError() error {
	f.errMu.Lock()
	defer f.errMu.Unlock()
	return f.err
}

func (f *NDJSONFormatter) encode(v any) {
	if err := json.NewEncoder(f.Writer).Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "ndjson encode error: %v\n", err)
		f.errMu.Lock()
		if f.err == nil {
			f.err = err
		}
		f.errMu.Unlock()
	}
}
