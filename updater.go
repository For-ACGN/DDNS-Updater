package ddns

import (
	"context"
	"io"
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

	ctx      context.Context
	cancel   context.CancelFunc
	runOnce  sync.Once
	stopOnce sync.Once
	mutex    sync.Mutex
	wg       sync.WaitGroup
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
	var ok bool
	defer func() {
		if ok {
			return
		}
		_ = logger.Close()
	}()
	if !cfg.PublicIPv4.Enable && !cfg.PublicIPv6.Enable {
		return nil, errors.New("IPv4/IPv6 are all disabled")
	}
	var (
		pubIPv4Req    *http.Request
		pubIPv6Req    *http.Request
		pubIPv4Client *http.Client
		pubIPv6Client *http.Client
	)
	if cfg.PublicIPv4.Enable {
		pubIPv4Req, pubIPv4Client, err = newIPv4HTTPClient(cfg)
		if err != nil {
			return nil, err
		}
		pubIPv4Client.Timeout = timeout
	}
	if cfg.PublicIPv6.Enable {
		pubIPv6Req, pubIPv6Client, err = newIPv6HTTPClient(cfg)
		if err != nil {
			return nil, err
		}
		pubIPv6Client.Timeout = timeout
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
	ok = true
	return &updater, nil
}

func newIPv4HTTPClient(cfg *Config) (*http.Request, *http.Client, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.PublicIPv4.URL, nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "invalid url about public ipv4 address provider")
	}
	proxy, err := readProxyURL(cfg.PublicIPv4.ProxyURL)
	if err != nil {
		return nil, nil, err
	}
	tr := &http.Transport{
		Proxy: proxy,
	}
	client := &http.Client{
		Transport: tr,
	}
	la := cfg.PublicIPv4.LocalAddr
	if la == "" {
		return req, client, nil
	}
	if net.ParseIP(la) != nil {
		la = net.JoinHostPort(la, "0")
	}
	lAddr, err := net.ResolveTCPAddr("tcp4", la)
	if err != nil {
		return nil, nil, errors.Wrap(err, "invalid local ipv4 address")
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
	return req, client, nil
}

func newIPv6HTTPClient(cfg *Config) (*http.Request, *http.Client, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.PublicIPv6.URL, nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "invalid url about public ipv6 address provider")
	}
	proxy, err := readProxyURL(cfg.PublicIPv6.ProxyURL)
	if err != nil {
		return nil, nil, err
	}
	tr := &http.Transport{
		Proxy: proxy,
	}
	client := &http.Client{
		Transport: tr,
	}
	la := cfg.PublicIPv6.LocalAddr
	if la == "" {
		return req, client, nil
	}
	if net.ParseIP(la) != nil {
		la = net.JoinHostPort(la, "0")
	}
	lAddr, err := net.ResolveTCPAddr("tcp6", la)
	if err != nil {
		return nil, nil, errors.Wrap(err, "invalid local ipv6 address")
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
	return req, client, nil
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
	updater.mutex.Lock()
	defer updater.mutex.Unlock()
	updater.runOnce.Do(func() {
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
			updater.Update()
		case <-updater.ctx.Done():
			return
		}
	}
}

func (updater *Updater) Update() {
	ipv4, err := updater.getPublicIPv4()
	if err != nil {
		updater.logger.Error("failed to get public ipv4 address:", err)
		return
	}
	ipv6, err := updater.getPublicIPv6()
	if err != nil {
		updater.logger.Error("failed to get public ipv6 address:", err)
		return
	}
	wg := sync.WaitGroup{}
	for i := 0; i < len(updater.providers); i++ {
		wg.Add(1)
		go func(p *provider) {
			defer wg.Done()
			updater.pushIP(p, ipv4, ipv6)
		}(updater.providers[i])
	}
	wg.Wait()
}

func (updater *Updater) getPublicIPv4() (string, error) {
	if updater.pubIPv4Req == nil {
		return "", nil
	}
	req := updater.pubIPv4Req.Clone(updater.ctx)
	resp, err := updater.pubIPv4Client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	ip := string(data)
	updater.logger.Info("IPv4:", ip)
	return ip, nil
}

func (updater *Updater) getPublicIPv6() (string, error) {
	if updater.pubIPv6Req == nil {
		return "", nil
	}
	req := updater.pubIPv6Req.Clone(updater.ctx)
	resp, err := updater.pubIPv6Client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	ip := string(data)
	updater.logger.Info("IPv6:", ip)
	return ip, nil
}

func (updater *Updater) pushIP(provider *provider, ipv4, ipv6 string) {
	if ipv4 != "" {
		err := updater.pushIPv4(provider, ipv4)
		if err != nil {
			updater.logger.Error("failed to push ipv4 address:", err)
		}
	}
	if ipv6 != "" {
		err := updater.pushIPv6(provider, ipv6)
		if err != nil {
			updater.logger.Error("failed to push ipv6 address:", err)
		}
	}
}

func (updater *Updater) pushIPv4(provider *provider, ipv4 string) error {
	req, err := provider.NewIPv4Request(updater.ctx, ipv4)
	if err != nil {
		return err
	}
	resp, err := updater.pushIPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	r := string(data)
	for i := 0; i < len(provider.Resp); i++ {
		if r == provider.Resp[i] {
			return nil
		}
	}
	return errors.Errorf("unexcepted response: %s", r)
}

func (updater *Updater) pushIPv6(provider *provider, ipv6 string) error {
	req, err := provider.NewIPv6Request(updater.ctx, ipv6)
	if err != nil {
		return err
	}
	resp, err := updater.pushIPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	r := string(data)
	for i := 0; i < len(provider.Resp); i++ {
		if r == provider.Resp[i] {
			return nil
		}
	}
	return errors.Errorf("unexcepted response: %s", r)
}

func (updater *Updater) Stop() {
	updater.mutex.Lock()
	defer updater.mutex.Unlock()
	updater.stopOnce.Do(func() {
		updater.cancel()
		updater.wg.Wait()
		_ = updater.logger.Close()
	})
}
