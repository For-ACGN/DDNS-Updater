package main

import (
	"bytes"
	"flag"
	"log"
	"os"

	"github.com/kardianos/service"
	"github.com/pelletier/go-toml/v2"

	"github.com/For-ACGN/DDNS-Updater"
)

var (
	cfgPath   string
	upOnce    bool
	install   bool
	uninstall bool
)

func init() {
	flag.StringVar(&cfgPath, "config", "config.toml", "configuration file path")
	flag.BoolVar(&upOnce, "once", false, "update domain once")
	flag.BoolVar(&install, "install", false, "install service")
	flag.BoolVar(&uninstall, "uninstall", false, "uninstall service")
	flag.Parse()
}

type config struct {
	ddns.Config

	Service struct {
		Name        string `toml:"name"`
		DisplayName string `toml:"display_name"`
		Description string `toml:"description"`
	} `toml:"service"`
}

func main() {
	cfgData, err := os.ReadFile(cfgPath) // #nosec
	checkError(err)
	decoder := toml.NewDecoder(bytes.NewReader(cfgData))
	decoder.DisallowUnknownFields()

	var cfg config
	err = decoder.Decode(&cfg)
	checkError(err)

	updater, err := ddns.NewUpdater(&cfg.Config)
	checkError(err)

	if upOnce {
		updater.Update()
		return
	}

	// initialize service
	program := program{updater: updater}
	svcConfig := service.Config{
		Name:        cfg.Service.Name,
		DisplayName: cfg.Service.DisplayName,
		Description: cfg.Service.Description,
	}
	svc, err := service.New(&program, &svcConfig)
	checkError(err)

	// switch operation
	switch {
	case install:
		err = svc.Install()
		if err != nil {
			log.Fatalln("failed to install service:", err)
		}
		log.Println("install service successfully")
	case uninstall:
		err = svc.Uninstall()
		if err != nil {
			log.Fatalln("failed to uninstall service:", err)
		}
		log.Println("uninstall service successfully")
	default:
		lg, err := svc.Logger(nil)
		checkError(err)
		err = svc.Run()
		if err != nil {
			_ = lg.Error(err)
		}
	}
}

func checkError(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

type program struct {
	updater *ddns.Updater
}

func (p *program) Start(service.Service) error {
	p.updater.Run()
	p.updater.Update()
	return nil
}

func (p *program) Stop(service.Service) error {
	p.updater.Stop()
	return nil
}
