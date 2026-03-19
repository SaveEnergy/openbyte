package stream_test

import (
	"time"

	"github.com/saveenergy/openbyte/internal/stream"
	"github.com/saveenergy/openbyte/pkg/types"
)

const (
	streamMissingID          = "missing"
	streamUnknownID          = "nonexistent"
	streamFixedID            = "fixed-id"
	testIPPrimary            = "10.0.0.1"
	testIPSecondary          = "10.0.0.2"
	expectedErrMissingStream = "expected error for missing stream"
	startStreamErrFmt        = "StartStream: %v"
)

func newTestManager() *stream.Manager {
	m := stream.NewManager(10, 5)
	m.Start()
	return m
}

func testConfig(dir types.Direction) types.StreamConfig {
	return types.StreamConfig{
		Protocol:  types.ProtocolTCP,
		Direction: dir,
		Duration:  10 * time.Second,
		Streams:   2,
		ClientIP:  "10.0.0.1",
	}
}
