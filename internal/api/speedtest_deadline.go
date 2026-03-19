package api

import (
	"net/http"
	"time"
)

func speedtestIdleDeadline(absoluteDeadline time.Time) time.Time {
	idleDeadline := time.Now().Add(speedtestIOIdleTimeout)
	if idleDeadline.After(absoluteDeadline) {
		return absoluteDeadline
	}
	return idleDeadline
}

func refreshReadDeadline(controller *http.ResponseController, absoluteDeadline time.Time) error {
	if controller == nil {
		return nil
	}
	return controller.SetReadDeadline(speedtestIdleDeadline(absoluteDeadline))
}

func refreshWriteDeadline(controller *http.ResponseController, absoluteDeadline time.Time) error {
	if controller == nil {
		return nil
	}
	return controller.SetWriteDeadline(speedtestIdleDeadline(absoluteDeadline))
}
