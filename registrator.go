package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	golog "github.com/op/go-logging"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/gliderlabs/pkg/usage"
	"github.com/gliderlabs/registrator/bridge"
	"github.com/gliderlabs/registrator/logging"
)

var log = golog.MustGetLogger("main")

var Version string

var versionChecker = usage.NewChecker("registrator", Version)

var hostIp = flag.String("ip", "", "IP for ports mapped to the host")
var internal = flag.Bool("internal", false, "Use internal ports instead of published ones")
var useIpFromLabel = flag.String("useIpFromLabel", "", "Use IP which is stored in a label assigned to the container")
var refreshInterval = flag.Int("ttl-refresh", 0, "Frequency with which service TTLs are refreshed")
var refreshTtl = flag.Int("ttl", 0, "TTL for services (default is no expiry)")
var forceTags = flag.String("tags", "", "Append tags for all registered services")
var resyncInterval = flag.Int("resync", 0, "Frequency with which services are resynchronized")
var deregister = flag.String("deregister", "always", "Deregister exited services \"always\" or \"on-success\"")
var retryAttempts = flag.Int("retry-attempts", 0, "Max retry attempts to establish a connection with the backend. Use -1 for infinite retries")
var retryInterval = flag.Int("retry-interval", 2000, "Interval (in millisecond) between retry-attempts.")
var cleanup = flag.Bool("cleanup", false, "Remove dangling services")
var requireLabel = flag.Bool("require-label", false, "Only register containers which have the SERVICE_REGISTER label, and ignore all others.")

func getopt(name, def string) string {
	if env := os.Getenv(name); env != "" {
		return env
	}
	return def
}

func assert(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		versionChecker.PrintVersion()
		os.Exit(0)
	}

	flag.Parse()

	logging.Configure()

	log.Infof("Starting registrator %s ...", Version)
	quit := make(chan struct{})
	defer func() {
		if err := recover(); err != nil {
			log.Fatalf("Panic Occured:", err)
		} else {
			close(quit)
			log.Critical("Docker event loop closed") // todo: reconnect?
		}
	}()

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s [options] <registry URI>\n\n", os.Args[0])
		flag.PrintDefaults()
		log.Error("Failed to start registrator, options were incorrect.")
	}

	if flag.NArg() != 1 {
		if flag.NArg() == 0 {
			fmt.Fprint(os.Stderr, "Missing required argument for registry URI.\n\n")
		} else {
			fmt.Fprintln(os.Stderr, "Extra unparsed arguments:")
			fmt.Fprintln(os.Stderr, " ", strings.Join(flag.Args()[1:], " "))
			fmt.Fprint(os.Stderr, "Options should come before the registry URI argument.\n\n")
		}
		flag.Usage()
		os.Exit(2)
	}

	if *hostIp != "" {
		// below IP regex was obtained from http://blog.markhatton.co.uk/2011/03/15/regular-expressions-for-ip-addresses-cidr-ranges-and-hostnames/
		ipRegEx, _ := regexp.Compile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
		if !ipRegEx.MatchString(*hostIp) {
			fmt.Fprintf(os.Stderr, "Invalid IP address '%s', please use a valid address.\n", *hostIp)
			os.Exit(2)
		}
		log.Debug("Forcing host IP to", *hostIp)
	}

	if *requireLabel {
		log.Info("SERVICE_REGISTER label is required to register containers.")
	}

	if (*refreshTtl == 0 && *refreshInterval > 0) || (*refreshTtl > 0 && *refreshInterval == 0) {
		assert(errors.New("-ttl and -ttl-refresh must be specified together or not at all"))
	} else if *refreshTtl > 0 && *refreshTtl <= *refreshInterval {
		assert(errors.New("-ttl must be greater than -ttl-refresh"))
	}

	if *retryInterval <= 0 {
		assert(errors.New("-retry-interval must be greater than 0"))
	}

	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		os.Setenv("DOCKER_HOST", "unix:///tmp/docker.sock")
	}

	docker, err := dockerapi.NewClientFromEnv()
	assert(err)

	if *deregister != "always" && *deregister != "on-success" {
		assert(errors.New("-deregister must be \"always\" or \"on-success\""))
	}
	log.Info("Creating Bridge")
	b, err := bridge.New(docker, flag.Arg(0), bridge.Config{
		HostIp:          *hostIp,
		Internal:        *internal,
		UseIpFromLabel:  *useIpFromLabel,
		ForceTags:       *forceTags,
		RefreshTtl:      *refreshTtl,
		RefreshInterval: *refreshInterval,
		DeregisterCheck: *deregister,
		Cleanup:         *cleanup,
		RequireLabel:    *requireLabel,
	})
	assert(err)
	log.Info("Bridge Created")

	attempt := 0
	for *retryAttempts == -1 || attempt <= *retryAttempts {
		log.Debugf("Connecting to backend (%v/%v)", attempt, *retryAttempts)

		err = b.Ping()
		if err == nil {
			break
		}

		if err != nil && attempt == *retryAttempts {
			assert(err)
		}

		time.Sleep(time.Duration(*retryInterval) * time.Millisecond)
		attempt++
	}

	// Start event listener before listing containers to avoid missing anything
	events := make(chan *dockerapi.APIEvents)
	assert(docker.AddEventListener(events))

	b.Sync(false)

	// Start a dead container pruning timer to allow refresh to work independently
	if *refreshInterval > 0 {
		ticker := time.NewTicker(time.Duration(*refreshInterval) * time.Second)
		go func() {
			for {
				select {
				case <-ticker.C:
					b.PruneDeadContainers()
				case <-quit:
					log.Debug("Quit message received. Exiting PruneDeadContainer loop")
					ticker.Stop()
					return
				}
			}
		}()
	}

	// Start the TTL refresh timer
	if *refreshInterval > 0 {
		ticker := time.NewTicker(time.Duration(*refreshInterval) * time.Second)
		go func() {
			for {
				select {
				case <-ticker.C:
					b.Refresh()
				case <-quit:
					log.Debug("Quit message received. Exiting Refresh loop")
					ticker.Stop()
					return
				}
			}
		}()
	}

	// Start the resync timer if enabled
	if *resyncInterval > 0 {
		resyncTicker := time.NewTicker(time.Duration(*resyncInterval) * time.Second)
		go func() {
			for {
				select {
				case <-resyncTicker.C:
					b.Sync(true)
				case <-quit:
					log.Debug("Quit message received. Exiting Resync loop")
					resyncTicker.Stop()
					return
				}
			}
		}()
	}

	// Process Docker events
	for msg := range events {
		switch msg.Status {
		case "start":
			log.Debugf("Docker Event Received: Start %s", msg.ID)
			go b.Add(msg.ID)
		case "die":
			log.Debugf("Docker Event Received: Die %s", msg.ID)
			go b.RemoveOnExit(msg.ID)
		}
	}
}
