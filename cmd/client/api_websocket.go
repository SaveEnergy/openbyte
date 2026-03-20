package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func streamMetrics(ctx context.Context, wsURL string, formatter OutputFormatter, config *Config) error {
	normalizedURL, err := normalizeWebSocketURL(config.ServerURL, wsURL)
	if err != nil {
		return err
	}
	wsURL = normalizedURL

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}
	defer conn.Close()

	readTimeout := 30 * time.Second
	if config.Timeout > 0 {
		readTimeout = time.Duration(config.Timeout) * time.Second
	}

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	receivedComplete := false
	for {
		conn.SetReadDeadline(time.Now().Add(readTimeout))

		var msg WebSocketMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return handleWebSocketReadError(ctx, err, receivedComplete)
		}

		done, processErr := processWebSocketMessage(formatter, &msg, &receivedComplete)
		if processErr != nil {
			return processErr
		}
		if done {
			return nil
		}
	}
}

func normalizeWebSocketURL(serverRaw, wsURL string) (string, error) {
	if strings.HasPrefix(wsURL, "/") {
		serverURL, err := url.Parse(serverRaw)
		if err != nil {
			return "", fmt.Errorf("parse server URL: %w", err)
		}
		wsScheme := "ws"
		if serverURL.Scheme == "https" {
			wsScheme = "wss"
		}
		return fmt.Sprintf("%s://%s%s", wsScheme, serverURL.Host, wsURL), nil
	}
	u, err := url.Parse(wsURL)
	if err != nil {
		return "", fmt.Errorf("parse websocket URL: %w", err)
	}
	if u.Scheme == "http" {
		u.Scheme = "ws"
	} else if u.Scheme == "https" {
		u.Scheme = "wss"
	}
	return u.String(), nil
}

func handleWebSocketReadError(ctx context.Context, err error, receivedComplete bool) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		if receivedComplete {
			return nil
		}
		return fmt.Errorf("websocket closed before completion message")
	}
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Errorf("websocket read timeout: %w", err)
	}
	return fmt.Errorf("read message: %w", err)
}

func processWebSocketMessage(formatter OutputFormatter, msg *WebSocketMessage, receivedComplete *bool) (bool, error) {
	switch msg.Type {
	case "progress":
		formatter.FormatProgress(msg.Progress, msg.ElapsedSeconds, msg.RemainingSeconds)
	case "metrics":
		if msg.Metrics != nil {
			formatter.FormatMetrics(msg.Metrics)
		}
	case "complete":
		*receivedComplete = true
		if msg.Results != nil {
			formatter.FormatComplete(msg.Results)
			return true, nil
		}
	case "error":
		return false, fmt.Errorf("test failed: %s", msg.Message)
	}
	return false, nil
}
