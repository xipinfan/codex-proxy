package netutil

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

// NormalizeResolveAddress normalizes resolve target input and supports:
// - host
// - host:port
// - https://host[:port][/path]
// - host[/path]
// It also removes trailing dot from FQDN.
func NormalizeResolveAddress(input string) string {
	v := strings.TrimSpace(input)
	if v == "" {
		return ""
	}

	if strings.Contains(v, "://") {
		if u, err := url.Parse(v); err == nil && u.Host != "" {
			v = u.Host
		}
	}

	if i := strings.Index(v, "/"); i >= 0 {
		v = v[:i]
	}
	v = strings.TrimSpace(v)
	v = strings.TrimSuffix(v, ".")
	return v
}

// BuildResolveDialContext returns a DialContext that redirects connections for targetHost
// to resolveAddress (host or host:port). If resolveAddress is empty, it returns dialer.DialContext.
func BuildResolveDialContext(dialer *net.Dialer, targetHost, resolveAddress string) func(context.Context, string, string) (net.Conn, error) {
	targetHost = strings.TrimSuffix(strings.TrimSpace(strings.ToLower(targetHost)), ".")
	resolveAddress = NormalizeResolveAddress(resolveAddress)
	if targetHost == "" || resolveAddress == "" {
		return dialer.DialContext
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return dialer.DialContext(ctx, network, addr)
		}
		if !strings.EqualFold(host, targetHost) {
			return dialer.DialContext(ctx, network, addr)
		}

		overrideAddr := resolveAddress
		if _, _, splitErr := net.SplitHostPort(resolveAddress); splitErr != nil {
			overrideAddr = net.JoinHostPort(resolveAddress, port)
		}
		return dialer.DialContext(ctx, network, overrideAddr)
	}
}

// BuildProxyDialContext 支持 HTTP/HTTPS/SOCKS5 代理
// 结合 DNS 解析和代理功能
// proxyURL 支持: http://host:port, https://host:port, socks5://host:port
// 支持代理认证: socks5://user:pass@host:port
func BuildProxyDialContext(dialer *net.Dialer, proxyURL, targetHost, resolveAddress string) func(context.Context, string, string) (net.Conn, error) {
	baseDialer := BuildResolveDialContext(dialer, targetHost, resolveAddress)

	if proxyURL == "" {
		return baseDialer
	}

	parsedURL, err := url.Parse(strings.TrimSpace(proxyURL))
	if err != nil {
		log.Warnf("代理 URL 解析失败: %v，将忽略代理配置", err)
		return baseDialer
	}

	scheme := strings.ToLower(parsedURL.Scheme)

	// HTTP/HTTPS 代理使用标准库支持，不需要特殊处理这里
	// 在 http.Transport 层面处理（通过 transport.Proxy）

	// SOCKS5 代理需要自定义 DialContext
	if scheme == "socks5" || scheme == "socks5h" {
		socksDialer, err := buildSOCKS5Dialer(dialer, parsedURL)
		if err != nil {
			log.Warnf("SOCKS5 代理创建失败: %v，将使用直连", err)
			return baseDialer
		}
		log.Infof("已启用 SOCKS5 代理: %s", parsedURL.Hostname())

		// 返回一个适配器函数，将 proxy.Dialer 适配为 DialContext
		return func(ctx context.Context, network, addr string) (net.Conn, error) {
			// proxy.Dialer 不支持 context，但我们尽力应用超时
			// 通过 dialer 本身的超时设置
			return socksDialer.Dial(network, addr)
		}
	}

	log.Debugf("代理方案 '%s' 由 http.Transport#Proxy 处理", scheme)
	return baseDialer
}

// buildSOCKS5Dialer 创建 SOCKS5 代理拨号器
// 支持认证: socks5://user:pass@host:port
func buildSOCKS5Dialer(baseDialer *net.Dialer, proxyURL *url.URL) (proxy.Dialer, error) {
	auth := &proxy.Auth{}
	if proxyURL.User != nil {
		auth.User = proxyURL.User.Username()
		if password, ok := proxyURL.User.Password(); ok {
			auth.Password = password
		}
	}

	var proxyDialer proxy.Dialer
	var err error

	// 如果 URL scheme 是 socks5h，表示 DNS 查询通过代理进行
	if proxyURL.Scheme == "socks5h" {
		proxyDialer, err = proxy.SOCKS5("tcp", proxyURL.Host, auth, baseDialer)
	} else {
		proxyDialer, err = proxy.SOCKS5("tcp", proxyURL.Host, auth, baseDialer)
	}

	if err != nil {
		return nil, fmt.Errorf("创建 SOCKS5 代理失败: %w", err)
	}

	return proxyDialer, nil
}
