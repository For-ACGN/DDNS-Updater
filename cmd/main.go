package main

import (
	"github.com/pelletier/go-toml/v2"
)

func main() {
	toml.NewDecoder(nil).DisallowUnknownFields()
}
