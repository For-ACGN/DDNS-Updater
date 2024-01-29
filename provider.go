package ddns

import (
	"bytes"
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
	ipv4Req   *http.Request
	ipv6Req   *http.Request
	response  []string
	separator string
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
	ipv4Req, err := parseIPv4Request(cfg, host)
	if err != nil {
		return nil, err
	}
	ipv6Req, err := parseIPv6Request(cfg, host)
	if err != nil {
		return nil, err
	}
	p := provider{
		ipv4Req:  ipv4Req,
		ipv6Req:  ipv6Req,
		response: strings.Split(cfg.Meta.Response, "|"),
	}
	return &p, nil
}

func parseIPv4Request(cfg *provCfg, host *url.URL) (*http.Request, error) {
	if cfg.IPv4.Path == "" {
		return nil, nil
	}
	var body io.Reader
	if cfg.IPv4.Body != "" {
		tmpl, err := template.New("ipv4").Parse(cfg.IPv4.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse ipv4 provider http body")
		}
		b := bytes.NewBuffer(make([]byte, 0, len(cfg.IPv4.Body)))
		err = tmpl.Execute(b, cfg.Args)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse ipv4 provider http body arguments")
		}
		body = b
	}
	URL := host.JoinPath(cfg.IPv4.Path)
	req, err := http.NewRequest(cfg.Meta.Method, URL.String(), body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build ipv4 provider http request")
	}
	return req, nil
}

func parseIPv6Request(cfg *provCfg, host *url.URL) (*http.Request, error) {
	if cfg.IPv6.Path == "" {
		return nil, nil
	}
	var body io.Reader
	if cfg.IPv6.Body != "" {
		tmpl, err := template.New("ipv6").Parse(cfg.IPv6.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse ipv6 provider http body")
		}
		b := bytes.NewBuffer(make([]byte, 0, len(cfg.IPv6.Body)))
		err = tmpl.Execute(b, cfg.Args)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse ipv6 provider http body arguments")
		}
		body = b
	}
	URL := host.JoinPath(cfg.IPv6.Path)
	req, err := http.NewRequest(cfg.Meta.Method, URL.String(), body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build ipv6 provider http request")
	}
	return req, nil
}
