package ddns

import (
	"time"
)

// Config contains DDNS updater configurations.
type Config struct {
	Period  duration `toml:"period"`
	Timeout duration `toml:"timeout"`
	LogFile string   `toml:"log_file"`

	PublicIPv4 struct {
		Enable    bool   `toml:"enable"`
		URL       string `toml:"url"`
		LocalAddr string `toml:"laddr"`
		Proxy     string `toml:"proxy"`
	} `toml:"public_ipv4"`

	PublicIPv6 struct {
		Enable    bool   `toml:"enable"`
		URL       string `toml:"url"`
		LocalAddr string `toml:"laddr"`
		Proxy     string `toml:"proxy"`
	} `toml:"public_ipv6"`

	Provider struct {
		Dir   string   `toml:"dir"`
		Item  []string `toml:"item"`
		Proxy string   `toml:"proxy"`
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
