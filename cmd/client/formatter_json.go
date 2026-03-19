package client

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/saveenergy/openbyte/pkg/types"
)

func (f *JSONFormatter) FormatProgress(progress, elapsed, remaining float64) {
	_ = progress
	_ = elapsed
	_ = remaining
}

func (f *JSONFormatter) FormatMetrics(metrics *types.Metrics) {
	_ = metrics
}

func (f *JSONFormatter) FormatComplete(results *StreamResults) {
	if err := json.NewEncoder(f.Writer).Encode(results); err != nil {
		fmt.Fprintf(os.Stderr, "openbyte client: error encoding JSON: %v\n", err)
	}
}

func (f *JSONFormatter) FormatError(err error) {
	errResp := JSONErrorResponse{
		SchemaVersion: SchemaVersion,
		Error:         true,
		Code:          classifyErrorCode(err),
		Message:       err.Error(),
	}
	if encErr := json.NewEncoder(f.Writer).Encode(errResp); encErr != nil {
		fmt.Fprintf(os.Stderr, "openbyte client: error encoding JSON: %v\n", encErr)
	}
}
