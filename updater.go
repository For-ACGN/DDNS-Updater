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
	logger    *logger
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
	period := time.Duration(cfg.Period)
	if period == 0 {
		period = defaultUpdatePeriod
	}
	timeout := time.Duration(cfg.Timeout)
	if timeout == 0 {
		timeout = defaultUpdateTimeout
	}
	logger, err := newLogger(cfg.LogFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create logger")
	}
	var (
		pubIPv4Req    *http.Request
		pubIPv6Req    *http.Request
		pubIPv4Client *http.Client
		pubIPv6Client *http.Client
	)
	if cfg.PublicIPv4.Enable {
		pubIPv4Req, err = http.NewRequest(http.MethodGet, cfg.PublicIPv4.URL, nil)
		if err != nil {
			return nil, errors.Wrap(err, "invalid url about public ipv4 address provider")
		}
		tr, err := setIPv4Transport(cfg)
		if err != nil {
			return nil, err
		}
		pubIPv4Client = &http.Client{
			Transport: tr,
			Timeout:   timeout,
		}
	}
	if cfg.PublicIPv6.Enable {
		pubIPv6Req, err = http.NewRequest(http.MethodGet, cfg.PublicIPv6.URL, nil)
		if err != nil {
			return nil, errors.Wrap(err, "invalid url about public ipv6 address provider")
		}
		tr, err := setIPv6Transport(cfg)
		if err != nil {
			return nil, err
		}
		pubIPv6Client = &http.Client{
			Transport: tr,
			Timeout:   timeout,
		}
	}
	providers, err := loadProviders(cfg)
	if err != nil {
		return nil, err
	}
	proxy, err := readProxyURL(cfg.Provider.ProxyURL)
	if err != nil {
		return nil, err
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
		logger:        logger,
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

func setIPv4Transport(cfg *Config) (*http.Transport, error) {
	proxy, err := readProxyURL(cfg.PublicIPv4.ProxyURL)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		Proxy: proxy,
	}
	la := cfg.PublicIPv4.LocalAddr
	if la == "" {
		return tr, nil
	}
	if net.ParseIP(la) != nil {
		la = net.JoinHostPort(la, "0")
	}
	lAddr, err := net.ResolveTCPAddr("tcp4", la)
	if err != nil {
		return nil, errors.Wrap(err, "invalid local ipv4 address")
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
	return tr, nil
}

func setIPv6Transport(cfg *Config) (*http.Transport, error) {
	proxy, err := readProxyURL(cfg.PublicIPv6.ProxyURL)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		Proxy: proxy,
	}
	la := cfg.PublicIPv6.LocalAddr
	if la == "" {
		return tr, nil
	}
	if net.ParseIP(la) != nil {
		la = net.JoinHostPort(la, "0")
	}
	lAddr, err := net.ResolveTCPAddr("tcp6", la)
	if err != nil {
		return nil, errors.Wrap(err, "invalid local ipv6 address")
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
	return tr, nil
}

func readProxyURL(URL string) (func(*http.Request) (*url.URL, error), error) {
	if URL == "" {
		return nil, nil
	}
	proxyURL, err := url.Parse(URL)
	if err != nil {
		return nil, errors.Wrap(err, "invalid proxy url")
	}
	proxy := func(*http.Request) (*url.URL, error) {
		return proxyURL, nil
	}
	return proxy, nil
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
