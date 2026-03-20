package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"
)

func parseDownloadParams(r *http.Request, maxDurationSec int) (time.Duration, int, error) {
	duration := 10 * time.Second
	durationRaw := r.URL.Query().Get("duration")
	if d, ok, err := parseOptionalIntInRange(durationRaw, 1, maxDurationSec, "duration must be 1-"+strconv.Itoa(maxDurationSec)); err != nil {
		return 0, 0, err
	} else if ok {
		duration = time.Duration(d) * time.Second
	}

	chunkSize := 1048576
	chunkRaw := r.URL.Query().Get("chunk")
	if c, ok, err := parseOptionalIntInRange(chunkRaw, 65536, 4194304, "chunk must be 65536-4194304"); err != nil {
		return 0, 0, err
	} else if ok {
		chunkSize = c
	}
	return duration, chunkSize, nil
}

func parseOptionalIntInRange(raw string, min, max int, errMessage string) (int, bool, error) {
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return 0, true, errors.New(errMessage)
	}
	return value, true, nil
}
