package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/jfrazelle/onion/pkg/dknet"
	"github.com/jfrazelle/onion/tor"
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
)

var (
	debug   bool
	version bool
)

func init() {
	// parse flags
	flag.BoolVar(&version, "version", false, "print version and exit")
	flag.BoolVar(&version, "v", false, "print version and exit (shorthand)")
	flag.BoolVar(&debug, "d", false, "run in debug mode")

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
	d, err := tor.NewDriver()
	if err != nil {
		logrus.Fatal(err)
	}
	h := dknet.NewHandler(d)
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
