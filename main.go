package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/pidfile"
	"github.com/docker/go-plugins-helpers/network"
	"github.com/jessfraz/onion/tor"
)

const (
	// BANNER is what is printed for help/info output
	BANNER = `             _
  ___  _ __ (_) ___  _ __
 / _ \| '_ \| |/ _ \| '_ \
| (_) | | | | | (_) | | | |
 \___/|_| |_|_|\___/|_| |_|

 Tor networking plugin for docker containers
 Version: %s

`
	// VERSION is the binary version.
	VERSION = "v0.1.0"

	defaultPidFile = "/var/run/onion.pid"
)

var (
	debug   bool
	version bool

	pidFile string
)

func init() {
	// parse flags
	flag.BoolVar(&version, "version", false, "print version and exit")
	flag.BoolVar(&version, "v", false, "print version and exit (shorthand)")
	flag.BoolVar(&debug, "d", false, "run in debug mode")

	flag.StringVar(&pidFile, "pidfile", defaultPidFile, "path to use for plugin's PID file")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(BANNER, VERSION))
		flag.PrintDefaults()
	}

	flag.Parse()

	if version {
		fmt.Printf("%s", VERSION)
		os.Exit(0)
	}

	// set log level
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
}

func main() {
	// setup the PID file if passed
	if pidFile != "" {
		pf, err := pidfile.New(pidFile)
		if err != nil {
			logrus.Fatalf("Error starting daemon: %v", err)
		}
		pfile := pf
		defer func() {
			if err := pfile.Remove(); err != nil {
				logrus.Error(err)
			}
		}()
	}

	d, err := tor.NewDriver()
	if err != nil {
		logrus.Fatal(err)
	}
	h := network.NewHandler(d)
	h.ServeUnix("root", "tor")
}

func usageAndExit(message string, exitCode int) {
	if message != "" {
		fmt.Fprintf(os.Stderr, message)
		fmt.Fprintf(os.Stderr, "\n\n")
	}
	flag.Usage()
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(exitCode)
}
