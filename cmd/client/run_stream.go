package client

import "context"

func runStream(ctx context.Context, config *Config, formatter OutputFormatter) error {
	return runHTTPStream(ctx, config, formatter)
}
