package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/pkg/profile"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"github.com/spf13/pflag"
	"github.com/trim21/errgo"
	_ "go.uber.org/automaxprocs"

	"tyr/internal/client"
	"tyr/internal/config"
	"tyr/internal/web"
)

func defaultSessionPath() string {
	h, err := os.UserHomeDir()
	if err != nil {
		panic(errgo.Wrap(err, "failed to get home directory, please set session path with --session-path manually"))
	}

	return filepath.Join(h, ".tyr")
}

func main() {
	var sessionPath = pflag.String("session-path", "", "client session path (default ~/.ve/)")
	var configFilePath = pflag.String("config-file", "", "path to config file (default {session-path}/config.toml)")
	var address = pflag.String("address", "127.0.0.1:8003", "web interface address")
	var p2pPort = pflag.Uint16("p2p-port", 0, "p2p listen port (default 50047)")

	var profiling = pflag.Bool("profile", false, "enable profiling for CPU and Memory")
	var profileCpu = pflag.Bool("profile-cpu", false, "enable CPU profiling only")
	var profileMem = pflag.Bool("profile-memory", false, "enable Memory profiling only")

	// this avoids 'pflag: help requested' error when calling for help message.
	if slices.Contains(os.Args[1:], "--help") || slices.Contains(os.Args[1:], "-h") {
		pflag.Usage()
		fmt.Println("\nNote: extra options will override config file, but won't change config file.")
		return
	}

	pflag.Parse()

	if *profileCpu || *profileMem || *profiling {
		var opt = make([]func(*profile.Profile), 2)
		if *profileCpu || *profiling {
			opt = append(opt, profile.CPUProfile)
		}
		if *profileMem || *profiling {
			opt = append(opt, profile.MemProfile)
		}
		defer profile.Start(opt...).Stop()
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

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

	lo.Must0(app.AddTorrent(lo.Must(metainfo.LoadFromFile(`C:\Users\Trim21\Downloads\ubuntu-24.04-desktop-amd64.iso.torrent`)), "C:\\Users\\Trim21\\Downloads"))

	go app.Start()

	server := web.New(app)

	fmt.Println("start", *address)
	err = http.ListenAndServe(*address, server)
	if err != nil {
		panic(err)
	}
}
