package ddns

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
)

const (
	defaultUpdatePeriod  = time.Minute
	defaultUpdateTimeout = 15 * time.Second
)

// Updater is a ddns updater, it will get public IPv4/IPv6
// addresses from public IP address service, then report
// them to the DDNS provider.
type Updater struct {
	pubIPv4 *url.URL
	pubIPv6 *url.URL
}

// NewUpdater is used to create a new ddns updater.
func NewUpdater(cfg *Config) (*Updater, error) {
	pubIPv4, err := url.Parse(cfg.PublicIPv4)
	if err != nil {
		return nil, errors.Wrap(err, "invalid url about public ipv4 address provider")
	}
	pubIPv6, err := url.Parse(cfg.PublicIPv6)
	if err != nil {
		return nil, errors.Wrap(err, "invalid url about public ipv6 address provider")
	}
	period := time.Duration(cfg.Period)
	if period == 0 {
		period = defaultUpdatePeriod
	}
	timeout := time.Duration(cfg.Timeout)
	if timeout == 0 {
		timeout = defaultUpdateTimeout
	}
	var proxy func(*http.Request) (*url.URL, error)
	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			return nil, errors.Wrap(err, "invalid proxy url")
		}
		proxy = func(*http.Request) (*url.URL, error) {
			return proxyURL, nil
		}
	}
	pubIPv4Tr := &http.Transport{
		Proxy: proxy,
	}
	err = setIPv4Transport(pubIPv4Tr, cfg)
	if err != nil {
		return nil, err
	}
	pubIPv6Tr := &http.Transport{
		Proxy: proxy,
	}
	err = setIPv6Transport(pubIPv6Tr, cfg)
	if err != nil {
		return nil, err
	}
	pubIPv4Client := http.Client{
		Transport: pubIPv4Tr,
		Timeout:   timeout,
	}
	pubIPv6Client := http.Client{
		Transport: pubIPv6Tr,
		Timeout:   timeout,
	}
	// pushClient :=

	return nil, nil
}

func setIPv4Transport(tr *http.Transport, cfg *Config) error {
	if cfg.LocalIPv4 != "" {
		return nil
	}
	proxy := tr.Proxy
	addr := net.JoinHostPort(cfg.LocalIPv4, "0")
	lAddr, err := net.ResolveTCPAddr("tcp4", addr)
	if err != nil {
		return errors.Wrap(err, "invalid local ipv4 address")
	}
	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := net.Dialer{
			LocalAddr: lAddr,
		}
		if proxy == nil {
			return dialer.DialContext(ctx, network, addr)
		}
		conn, err := dialer.DialContext(ctx, network, addr)
		if err == nil {
			return conn, nil
		}
		dialer.LocalAddr = nil
		return dialer.DialContext(ctx, network, addr)
	}
	tr.DialContext = dialContext
	return nil
}

func setIPv6Transport(tr *http.Transport, cfg *Config) error {
	if cfg.LocalIPv6 != "" {
		return nil
	}
	proxy := tr.Proxy
	addr := net.JoinHostPort(cfg.LocalIPv6, "0")
	lAddr, err := net.ResolveTCPAddr("tcp6", addr)
	if err != nil {
		return errors.Wrap(err, "invalid local ipv6 address")
	}
	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := net.Dialer{
			LocalAddr: lAddr,
		}
		if proxy == nil {
			return dialer.DialContext(ctx, network, addr)
		}
		conn, err := dialer.DialContext(ctx, network, addr)
		if err == nil {
			return conn, nil
		}
		dialer.LocalAddr = nil
		return dialer.DialContext(ctx, network, addr)
	}
	tr.DialContext = dialContext
	return nil
}
