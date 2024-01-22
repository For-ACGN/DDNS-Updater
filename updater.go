package ddns

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
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
	period    time.Duration
	providers []*provider

	pubIPv4Req    *http.Request
	pubIPv6Req    *http.Request
	pubIPv4Client *http.Client
	pubIPv6Client *http.Client
	pushIPClient  *http.Client

	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
	wg     sync.WaitGroup
}

// NewUpdater is used to create a new ddns updater.
func NewUpdater(cfg *Config) (*Updater, error) {
	pubIPv4Req, err := http.NewRequest(http.MethodGet, cfg.PublicIPv4, nil)
	if err != nil {
		return nil, errors.Wrap(err, "invalid url about public ipv4 address provider")
	}
	pubIPv6Req, err := http.NewRequest(http.MethodGet, cfg.PublicIPv6, nil)
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
	providers, err := loadProviders(cfg)
	if err != nil {
		return nil, err
	}
	pubIPv4Client := &http.Client{
		Transport: pubIPv4Tr,
		Timeout:   timeout,
	}
	pubIPv6Client := &http.Client{
		Transport: pubIPv6Tr,
		Timeout:   timeout,
	}
	tr := &http.Transport{
		Proxy: proxy,
	}
	pushIPClient := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}
	updater := Updater{
		period:        period,
		providers:     providers,
		pubIPv4Req:    pubIPv4Req,
		pubIPv6Req:    pubIPv6Req,
		pubIPv4Client: pubIPv4Client,
		pubIPv6Client: pubIPv6Client,
		pushIPClient:  pushIPClient,
	}
	updater.ctx, updater.cancel = context.WithCancel(context.Background())
	return &updater, nil
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

func loadProviders(cfg *Config) ([]*provider, error) {
	l := len(cfg.Provider.Item)
	if l < 0 {
		return nil, errors.New("empty provider")
	}
	providers := make([]*provider, 0, l)
	for i := 0; i < l; i++ {
		path := filepath.Join(cfg.Provider.Dir, cfg.Provider.Item[i])
		provider, err := loadProvider(path)
		if err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	return providers, nil
}

func loadProvider(path string) (*provider, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read provider config file")
	}
	defer func() { _ = file.Close() }()
	provider, err := newProvider(file)
	if err != nil {
		return nil, err
	}
	return provider, nil
}

func (updater *Updater) Run() {
	updater.once.Do(func() {
		updater.wg.Add(1)
		go updater.run()
	})
}

func (updater *Updater) run() {
	defer updater.wg.Done()
	ticker := time.NewTicker(updater.period)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := updater.Update()
			if err != nil {

			}
		case <-updater.ctx.Done():
			return
		}
	}
}

func (updater *Updater) Update() error {

	return nil
}

func (updater *Updater) Stop() {
	updater.cancel()
	updater.wg.Wait()
}
