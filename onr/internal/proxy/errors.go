package proxy

import (
	"context"
	"errors"
	"net"
	"strings"
	"syscall"
)

func isClientDisconnectErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	// Common write-side errors when the downstream client closes the connection mid-stream.
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	var op *net.OpError
	if errors.As(err, &op) {
		if errors.Is(op.Err, syscall.EPIPE) || errors.Is(op.Err, syscall.ECONNRESET) {
			return true
		}
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "broken pipe") || strings.Contains(s, "connection reset by peer")
}
