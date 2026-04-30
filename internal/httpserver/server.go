package httpserver

import "github.com/valyala/fasthttp"

func New(handler fasthttp.RequestHandler, opts Options) *fasthttp.Server {
	return &fasthttp.Server{
		Handler:            handler,
		Name:               opts.Name,
		DisableKeepalive:   false,
		Concurrency:        opts.Concurrency,
		IdleTimeout:        opts.IdleTimeout,
		ReadTimeout:        0,
		WriteTimeout:       0,
		HeaderReceived:     requestConfig(opts),
		TCPKeepalive:       opts.TCPKeepalive,
		TCPKeepalivePeriod: opts.TCPKeepalivePeriod,
		ReadBufferSize:     opts.ReadBufferSize,
		MaxRequestBodySize: opts.MaxRequestBodySize,
		MaxConnsPerIP:      0,
		MaxRequestsPerConn: 0,
		ErrorHandler:       parseErrorHandler,
	}
}

func requestConfig(opts Options) func(*fasthttp.RequestHeader) fasthttp.RequestConfig {
	return func(_ *fasthttp.RequestHeader) fasthttp.RequestConfig {
		return fasthttp.RequestConfig{
			ReadTimeout: opts.ListenReadHeaderTimeout,
		}
	}
}
