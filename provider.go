package ddns

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"text/template"

	"github.com/pelletier/go-toml/v2"
	"github.com/pkg/errors"
)

type provCfg struct {
	Meta struct {
		Host     string `toml:"host_url"`
		Method   string `toml:"method"`
		Response string `toml:"response"`
	} `toml:"meta"`

	IPv4 struct {
		Path string `toml:"path"`
		Body string `toml:"body"`
	} `toml:"ipv4"`

	IPv6 struct {
		Path string `toml:"path"`
		Body string `toml:"body"`
	} `toml:"ipv6"`

	Args map[string]string `toml:"args"`
}

type provider struct {
	cfg  *provCfg
	host *url.URL
	Resp []string
}

func newProvider(r io.Reader) (*provider, error) {
	d := toml.NewDecoder(r)
	d.DisallowUnknownFields()
	cfg := new(provCfg)
	err := d.Decode(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read provider")
	}
	tmpl, err := template.New("provider").Parse(cfg.Meta.Host)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse provider http url host")
	}
	hostBuf := bytes.NewBuffer(make([]byte, 0, len(cfg.Meta.Host)))
	err = tmpl.Execute(hostBuf, cfg.Args)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse provider http url host arguments")
	}
	host, err := url.Parse(hostBuf.String())
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse host")
	}
	if cfg.IPv4.Path == "" && cfg.IPv6.Path == "" {
		return nil, errors.New("IPv4/IPv6 url path are all empty")
	}
	p := provider{
		cfg:  cfg,
		host: host,
		Resp: strings.Split(cfg.Meta.Response, "|"),
	}
	return &p, nil
}

func (p *provider) NewIPv4Request(ctx context.Context, ip string) (*http.Request, error) {
	if p.cfg.IPv4.Path == "" {
		return nil, nil
	}
	p.cfg.Args["ipv4"] = ip
	tmpl, err := template.New("ipv4").Parse(p.cfg.IPv4.Path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse ipv4 provider http path")
	}
	b := bytes.NewBuffer(make([]byte, 0, len(p.cfg.IPv4.Path)))
	err = tmpl.Execute(b, p.cfg.Args)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse ipv4 provider http path arguments")
	}
	path := b.String()
	var body io.Reader
	if p.cfg.IPv4.Body != "" {
		tmpl, err = template.New("ipv4").Parse(p.cfg.IPv4.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse ipv4 provider http body")
		}
		b = bytes.NewBuffer(make([]byte, 0, len(p.cfg.IPv4.Body)))
		err = tmpl.Execute(b, p.cfg.Args)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse ipv4 provider http body arguments")
		}
		body = b
	}
	URL := p.host.JoinPath(path)
	req, err := http.NewRequestWithContext(ctx, p.cfg.Meta.Method, URL.String(), body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build ipv4 provider http request")
	}
	return req, nil
}

func (p *provider) NewIPv6Request(ctx context.Context, ip string) (*http.Request, error) {
	if p.cfg.IPv6.Path == "" {
		return nil, nil
	}
	p.cfg.Args["ipv6"] = ip
	tmpl, err := template.New("ipv6").Parse(p.cfg.IPv6.Path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse ipv6 provider http path")
	}
	b := bytes.NewBuffer(make([]byte, 0, len(p.cfg.IPv6.Path)))
	err = tmpl.Execute(b, p.cfg.Args)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse ipv6 provider http path arguments")
	}
	path := b.String()
	var body io.Reader
	if p.cfg.IPv6.Body != "" {
		tmpl, err = template.New("ipv6").Parse(p.cfg.IPv6.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse ipv6 provider http body")
		}
		b = bytes.NewBuffer(make([]byte, 0, len(p.cfg.IPv6.Body)))
		err = tmpl.Execute(b, p.cfg.Args)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse ipv6 provider http body arguments")
		}
		body = b
	}
	URL := p.host.JoinPath(path)
	req, err := http.NewRequestWithContext(ctx, p.cfg.Meta.Method, URL.String(), body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build ipv6 provider http request")
	}
	return req, nil
}
