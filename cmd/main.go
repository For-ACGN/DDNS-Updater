package main

import (
	"bytes"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/pelletier/go-toml/v2"

	"github.com/For-ACGN/DDNS-Updater"
)

var (
	cfgPath string
	upOnce  bool
)

func init() {
	flag.StringVar(&cfgPath, "config", "config.toml", "configuration file path")
	flag.BoolVar(&upOnce, "once", false, "update domain once")
	flag.Parse()
}

func main() {
	cfgData, err := os.ReadFile(cfgPath) // #nosec
	checkError(err)
	decoder := toml.NewDecoder(bytes.NewReader(cfgData))
	decoder.DisallowUnknownFields()

	var config ddns.Config
	err = decoder.Decode(&config)
	checkError(err)

	updater, err := ddns.NewUpdater(&config)
	checkError(err)

	if upOnce {
		updater.Update()
		return
	}

	updater.Run()
	updater.Update()

	// stop signal
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	<-signalCh

	updater.Stop()
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
