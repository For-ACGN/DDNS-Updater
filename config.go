package ddns

import (
	"time"
)

// Config contains DDNS updater configurations.
type Config struct {
	PublicIPv4 string   `toml:"pub_ipv4"`
	PublicIPv6 string   `toml:"pub_ipv6"`
	Period     duration `toml:"period"`
	Timeout    duration `toml:"timeout"`
	ProxyURL   string   `toml:"proxy_url"`

	Provider struct {
		Dir  string   `toml:"dir"`
		Item []string `toml:"item"`
	} `toml:"provider"`
}

// duration is patch for toml v2.
type duration time.Duration

// MarshalText implement encoding.TextMarshaler.
func (d duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// UnmarshalText implement encoding.TextUnmarshaler.
func (d *duration) UnmarshalText(b []byte) error {
	x, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}
	*d = duration(x)
	return nil
}
