package diagnostic

// Interpret produces a diagnostic Interpretation from raw metrics.
func Interpret(p Params) *Interpretation {
	interp := &Interpretation{
		SuitableFor: []string{},
		Concerns:    []string{},
	}

	interp.LatencyRating = rateLatency(p.LatencyMs)
	interp.SpeedRating = rateSpeed(p.DownloadMbps, p.UploadMbps)
	interp.StabilityRating = rateStability(p.JitterMs, p.PacketLoss)

	interp.SuitableFor = suitability(p)
	interp.Concerns = concerns(p)

	interp.Grade = computeGrade(interp.LatencyRating, interp.SpeedRating, interp.StabilityRating)
	interp.Summary = buildSummary(interp.Grade, p)

	return interp
}
