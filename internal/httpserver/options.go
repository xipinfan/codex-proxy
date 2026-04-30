package httpserver

import "time"

type Options struct {
	Name                    string
	Concurrency             int
	IdleTimeout             time.Duration
	ListenReadHeaderTimeout time.Duration
	TCPKeepalive            bool
	TCPKeepalivePeriod      time.Duration
	ReadBufferSize          int
	MaxRequestBodySize      int
}
