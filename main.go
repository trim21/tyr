package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/pflag"
	"github.com/trim21/errgo"

	"ve/internal/client"
	"ve/internal/config"
	"ve/internal/web"
)

func defaultSessionPath() string {
	h, err := os.UserHomeDir()
	if err != nil {
		panic(errgo.Wrap(err, "failed to get home directory, please set session path with --session-path manually"))
	}

	return filepath.Join(h, ".ve")
}

func main() {
	var sessionPath = pflag.String("session-path", "", "client session path (default ~/.ve/)")
	var configFilePath = pflag.String("config-file", "", "path to config file (default {session-path}/config.toml)")
	var address = pflag.String("address", "127.0.0.1:8003", "web interface address")
	var p2pPort = pflag.Uint16("p2p-port", 0, "p2p listen port (default 50047)")

	// this avoid 'pflag: help requested' error when calling for help message.
	if slices.Contains(os.Args[1:], "--help") || slices.Contains(os.Args[1:], "-h") {
		pflag.Usage()
		fmt.Println("\nNote: extra options will override config file, but won't change config file.")
		return
	}

	pflag.Parse()

	if *sessionPath == "" {
		*sessionPath = defaultSessionPath()
	}

	if *configFilePath == "" {
		*configFilePath = filepath.Join(*sessionPath, "config.toml")
	}

	if err := os.MkdirAll(*sessionPath, os.ModePerm); err != nil {
		panic(errgo.Wrap(err, "failed to create session path"))
	}

	cfg, err := config.LoadFromFile(*configFilePath)
	if err != nil {
		print(errgo.Wrap(err, "failed to load config"))
	}

	if *p2pPort != 0 {
		cfg.App.P2PPort = *p2pPort
	}

	app := client.New(cfg)

	go app.Start()

	server := web.New(app)

	log.Println("start", *address)
	log.Fatalln(http.ListenAndServe(*address, server))
}
