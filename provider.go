package ddns

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"text/template"

	"github.com/pelletier/go-toml/v2"
	"github.com/pkg/errors"
)

type provCfg struct {
	Meta struct {
		URL    string `toml:"http_url"`
		Body   string `toml:"http_body"`
		Method string `toml:"http_method"`
		Resp   string `toml:"http_resp"`
	} `toml:"meta"`
	Args map[string]string `toml:"args"`
}

type provider struct {
	req  *http.Request
	resp []string
}

func newProvider(r io.Reader) (*provider, error) {
	d := toml.NewDecoder(r)
	d.DisallowUnknownFields()
	cfg := provCfg{}
	err := d.Decode(&cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read provider")
	}
	tmpl, err := template.New("provider").Parse(cfg.Meta.URL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse provider http url")
	}
	URL := bytes.NewBuffer(make([]byte, 0, len(cfg.Meta.URL)))
	err = tmpl.Execute(URL, cfg.Args)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse provider arguments")
	}
	var body io.Reader
	if cfg.Meta.Body != "" {
		tmpl, err = template.New("provider").Parse(cfg.Meta.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse provider http body")
		}
		b := bytes.NewBuffer(make([]byte, 0, len(cfg.Meta.Body)))
		err = tmpl.Execute(b, cfg.Args)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse provider arguments")
		}
		body = b
	}
	req, err := http.NewRequest(cfg.Meta.Method, URL.String(), body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build provider http request")
	}
	p := provider{
		req:  req,
		resp: strings.Split(cfg.Meta.Resp, "|"),
	}
	return &p, nil
}
