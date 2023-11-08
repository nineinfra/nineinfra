package controller

import (
	"context"
	"github.com/go-logr/logr"
	"time"
)

func GiB2Bytes(datavolume int) int {
	return datavolume * 1024 * 1024 * 1024
}

func LogInfoInterval(ctx context.Context, interval int, msg string) {
	logger, _ := logr.FromContext(ctx)
	if interval < 5 {
		interval = 5
	}
	if time.Now().Second()%interval == 0 {
		logger.Info(msg)
	}
}

func LogInfo(ctx context.Context, msg string) {
	logger, _ := logr.FromContext(ctx)
	logger.Info(msg)
}
