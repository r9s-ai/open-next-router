package proxy

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"
)

func TestIsClientDisconnectErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "context_canceled", err: context.Canceled, want: true},
		{name: "epipe", err: syscall.EPIPE, want: true},
		{name: "econnreset", err: syscall.ECONNRESET, want: true},
		{name: "net_op_epipe", err: &net.OpError{Err: syscall.EPIPE}, want: true},
		{name: "net_op_econnreset", err: &net.OpError{Err: syscall.ECONNRESET}, want: true},
		{name: "broken_pipe_string", err: errors.New("write tcp 127.0.0.1: broken pipe"), want: true},
		{name: "other", err: errors.New("something else"), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isClientDisconnectErr(tc.err); got != tc.want {
				t.Fatalf("got %v, want %v (err=%v)", got, tc.want, tc.err)
			}
		})
	}
}
