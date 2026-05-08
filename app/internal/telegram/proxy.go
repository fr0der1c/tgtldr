package telegram

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gotd/td/telegram/dcs"
	"golang.org/x/net/proxy"
)

const (
	telegramProxyURLEnv = "TGTLDR_TELEGRAM_PROXY_URL"
	proxyDialTimeout    = 30 * time.Second
)

func telegramProxyResolverFromEnv() (dcs.Resolver, error) {
	raw := strings.TrimSpace(os.Getenv(telegramProxyURLEnv))
	if raw == "" {
		return nil, nil
	}

	resolver, err := telegramProxyResolver(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", telegramProxyURLEnv, err)
	}
	return resolver, nil
}

func telegramProxyResolver(raw string) (dcs.Resolver, error) {
	dial, err := telegramProxyDial(raw)
	if err != nil {
		return nil, err
	}
	return dcs.Plain(dcs.PlainOptions{Dial: dial}), nil
}

func telegramProxyDial(raw string) (dcs.DialFunc, error) {
	proxyURL, err := parseTelegramProxyURL(raw)
	if err != nil {
		return nil, err
	}

	dialer, err := proxy.FromURL(proxyURL, timeoutProxyDialer{timeout: proxyDialTimeout})
	if err != nil {
		return nil, fmt.Errorf("create proxy dialer: %w", err)
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialProxyWithContext(ctx, dialer, network, addr)
	}, nil
}

func parseTelegramProxyURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("proxy url is empty")
	}

	proxyURL, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parse proxy url: %w", err)
	}
	if proxyURL.Host == "" {
		return nil, fmt.Errorf("proxy url must include host")
	}
	switch proxyURL.Scheme {
	case "socks5", "socks5h":
		return proxyURL, nil
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q, use socks5 or socks5h", proxyURL.Scheme)
	}
}

type timeoutProxyDialer struct {
	timeout time.Duration
}

func (d timeoutProxyDialer) Dial(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, d.timeout)
}

type proxyDialResult struct {
	conn net.Conn
	err  error
}

func dialProxyWithContext(ctx context.Context, dialer proxy.Dialer, network, addr string) (net.Conn, error) {
	results := make(chan proxyDialResult)
	done := make(chan struct{})

	go func() {
		conn, err := dialer.Dial(network, addr)
		result := proxyDialResult{conn: conn, err: err}

		select {
		case results <- result:
		case <-done:
			if conn != nil {
				_ = conn.Close()
			}
		}
	}()

	select {
	case result := <-results:
		close(done)
		if result.err != nil {
			return nil, result.err
		}
		return result.conn, nil
	case <-ctx.Done():
		close(done)
		return nil, ctx.Err()
	}
}
