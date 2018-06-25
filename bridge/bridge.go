package bridge

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	dockerapi "github.com/fsouza/go-dockerclient"
)

const Timeout time.Duration = time.Duration(5 * time.Second)

var serviceIDPattern = regexp.MustCompile(`^(.+?):([a-zA-Z0-9][a-zA-Z0-9_.-]+):[0-9]+(?::udp)?$`)

type Bridge struct {
	sync.Mutex
	registry       RegistryAdapter
	docker         *dockerapi.Client
	services       map[string][]*Service
	deadContainers map[string]*DeadContainer
	config         Config
}

func New(docker *dockerapi.Client, adapterUri string, config Config) (*Bridge, error) {
	uri, err := url.Parse(adapterUri)
	if err != nil {
		return nil, errors.New("bad adapter uri: " + adapterUri)
	}
	factory, found := AdapterFactories.Lookup(uri.Scheme)
	if !found {
		return nil, errors.New("unrecognized adapter: " + adapterUri)
	}

	log.Debug("Using", uri.Scheme, "adapter:", uri)
	return &Bridge{
		docker:         docker,
		config:         config,
		registry:       factory.New(uri),
		services:       make(map[string][]*Service),
		deadContainers: make(map[string]*DeadContainer),
	}, nil
}

func (b *Bridge) Ping() error {
	return b.registry.Ping()
}

func (b *Bridge) Add(containerId string) {
	b.add(containerId, false)
}

func (b *Bridge) Remove(containerId string) {
	b.remove(containerId, true)
}

func (b *Bridge) RemoveOnExit(containerId string) {
	b.remove(containerId, b.shouldRemove(containerId))
}

func (b *Bridge) Refresh() {
	// Set hostIp if needed
	if b.config.HostIp == "" && b.config.IpServer != "" {
		b.config.HostIp = b.getIpFromServer()
		log.Debug("Refresh: setting HostIp to", b.config.HostIp)
	}

	for containerId, services := range b.getServicesCopy() {
		for _, service := range services {
			err := b.registry.Refresh(service)
			if err != nil {
				log.Error("refresh failed:", service.ID, err)
				continue
			}
			log.Debug("refreshed:", containerId[:12], service.ID)
		}
	}
}

func (b *Bridge) PruneDeadContainers() {
	b.Lock()
	defer b.Unlock()
	for containerId, deadContainer := range b.deadContainers {
		deadContainer.TTL -= b.config.RefreshInterval
		if deadContainer.TTL <= 0 {
			delete(b.deadContainers, containerId)
		}
	}
}

// Get a deep copy of the current services in a thread-safe way
func (b *Bridge) getServicesCopy() map[string][]*Service {
	b.Lock()
	defer b.Unlock()
	svcsCopy := make(map[string][]*Service)

	for id, svc := range b.services {
		var svcPointers []*Service
		for _, s := range svc {
			t := *s
			z := &t
			svcPointers = append(svcPointers, z)
		}
		svcsCopy[id] = svcPointers
	}
	return svcsCopy
}

func (b *Bridge) Sync(quiet bool) {
	// Set hostIp if needed
	if b.config.HostIp == "" && b.config.IpServer != "" {
		b.config.HostIp = b.getIpFromServer()
		log.Debug("Sync: setting HostIp to", b.config.HostIp)
	}

	// Take this to avoid having to use a mutex
	servicesSnapshot := b.getServicesCopy()

	containers, err := b.docker.ListContainers(dockerapi.ListContainersOptions{})
	if err != nil && quiet {
		log.Error("error listing containers, skipping sync")
		return
	} else if err != nil && !quiet {
		log.Fatal(err)
	}

	log.Debugf("Syncing services on %d containers", len(containers))

	// NOTE: This assumes reregistering will do the right thing, i.e. nothing..
	for _, listing := range containers {
		services := servicesSnapshot[listing.ID]
		if services == nil {
			go b.add(listing.ID, quiet)
		} else {
			for _, service := range services {
				err := b.registry.Register(service)
				if err != nil {
					log.Debug("sync register failed:", service, err)
				}
			}
		}
	}

	// Clean up services that were registered previously, but aren't
	// acknowledged within registrator
	if b.config.Cleanup {
		// Remove services if its corresponding container is not running
		log.Debug("Listing non-exited containers")
		filters := map[string][]string{"status": {"created", "restarting", "running", "paused"}}
		nonExitedContainers, err := b.docker.ListContainers(dockerapi.ListContainersOptions{Filters: filters})
		if err != nil {
			log.Debug("error listing nonExitedContainers, skipping sync", err)
			return
		}
		for listingId, _ := range servicesSnapshot {
			found := false
			for _, container := range nonExitedContainers {
				if listingId == container.ID {
					found = true
					break
				}
			}
			// This is a container that does not exist
			if !found {
				log.Debugf("stale: Removing service %s because it does not exist", listingId)
				go b.RemoveOnExit(listingId)
			}
		}

		log.Debug("Cleaning up dangling services")
		extServices, err := b.registry.Services()
		if err != nil {
			log.Error("cleanup failed:", err)
			return
		}

	Outer:
		for _, extService := range extServices {
			matches := serviceIDPattern.FindStringSubmatch(extService.ID)
			if len(matches) != 3 {
				// There's no way this was registered by us, so leave it
				continue
			}
			serviceHostname := matches[1]
			if serviceHostname != Hostname {
				// ignore because registered on a different host
				continue
			}
			serviceContainerName := matches[2]
			for _, listing := range servicesSnapshot {
				for _, service := range listing {
					if service.Name == extService.Name && serviceContainerName == service.Origin.container.Name[1:] {
						continue Outer
					}
				}
			}
			log.Debug("dangling:", extService.ID)
			err := b.registry.Deregister(extService)
			if err != nil {
				log.Error("deregister failed:", extService.ID, err)
				continue
			}
			log.Infof("During cleanup dangling %s removed", extService.ID)
		}
	}
}

func (b *Bridge) deleteDeadContainer(containerId string) {
	b.Lock()
	defer b.Unlock()

	if d := b.deadContainers[containerId]; d != nil {
		b.services[containerId] = d.Services
		delete(b.deadContainers, containerId)
	}
}

func (b *Bridge) appendService(containerId string, service *Service) {
	b.Lock()
	defer b.Unlock()
	if b.services[containerId] != nil {
		log.Debug("container, ", containerId[:12], ", already exists, will not append.")
		return
	}
	b.services[containerId] = append(b.services[containerId], service)
	log.Debug("added:", containerId[:12], service.ID)
}

func (b *Bridge) add(containerId string, quiet bool) {
	b.deleteDeadContainer(containerId)

	b.Lock()
	if b.services[containerId] != nil {
		log.Debug("container, ", containerId[:12], ", already exists, ignoring")
		// Alternatively, remove and readd or resubmit.
		return
	}
	b.Unlock()

	container, err := b.docker.InspectContainer(containerId)
	if err != nil {
		log.Error("unable to inspect container:", containerId[:12], err)
		return
	}

	ports := make(map[string]ServicePort)

	// Extract configured host port mappings, relevant when using --net=host
	for port, _ := range container.Config.ExposedPorts {
		published := []dockerapi.PortBinding{{"0.0.0.0", port.Port()}}
		ports[string(port)] = servicePort(container, port, published)
	}

	// Extract runtime port mappings, relevant when using --net=bridge
	for port, published := range container.NetworkSettings.Ports {
		ports[string(port)] = servicePort(container, port, published)
	}

	if len(ports) == 0 && !quiet {
		log.Debug("ignored:", container.ID[:12], "no published ports")
		return
	}

	servicePorts := make(map[string]ServicePort)
	for key, port := range ports {
		if b.config.Internal != true && port.HostPort == "" {
			if !quiet {
				log.Debug("ignored:", container.ID[:12], "port", port.ExposedPort, "not published on host")
			}
			continue
		}
		servicePorts[key] = port
	}

	isGroup := len(servicePorts) > 1
	for _, port := range servicePorts {
		service := b.newService(port, isGroup)
		if service == nil {
			if !quiet {
				log.Debug("ignored:", container.ID[:12], "service on port", port.ExposedPort)
			}
			continue
		}
		err := b.registry.Register(service)
		if err != nil {
			log.Error("register failed:", service, err)
			continue
		}
		b.appendService(container.ID, service)
	}
}

func (b *Bridge) newService(port ServicePort, isgroup bool) *Service {
	container := port.container
	defaultName := strings.Split(path.Base(container.Config.Image), ":")[0]

	// not sure about this logic. kind of want to remove it.
	hostname := Hostname
	if hostname == "" {
		hostname = port.HostIP
	}
	if port.HostIP == "0.0.0.0" {
		ip, err := net.ResolveIPAddr("ip", hostname)
		if err == nil {
			port.HostIP = ip.String()
		}
	}

	if b.config.HostIp != "" {
		port.HostIP = b.config.HostIp
	}

	metadata, metadataFromPort := serviceMetaData(container.Config, port.ExposedPort)

	ignore := mapDefault(metadata, "ignore", "")
	if ignore != "" {
		return nil
	}

	if b.config.RequireLabel {
		if strings.ToLower(mapDefault(metadata, "register", "false")) == "false" {
			log.Debugf("Did not find label SERVICE_REGISTER on %s - ignoring.", container.Name)
			return nil
		}
	}

	service := new(Service)
	service.Origin = port
	service.ID = hostname + ":" + container.Name[1:] + ":" + port.ExposedPort
	service.Name = mapDefault(metadata, "name", defaultName)
	if isgroup && !metadataFromPort["name"] {
		service.Name += "-" + port.ExposedPort
	}
	var p int

	if b.config.Internal == true {
		service.IP = port.ExposedIP
		p, _ = strconv.Atoi(port.ExposedPort)
	} else {
		service.IP = port.HostIP
		p, _ = strconv.Atoi(port.HostPort)
	}
	service.Port = p

	if b.config.UseIpFromLabel != "" {
		containerIp := container.Config.Labels[b.config.UseIpFromLabel]
		if containerIp != "" {
			slashIndex := strings.LastIndex(containerIp, "/")
			if slashIndex > -1 {
				service.IP = containerIp[:slashIndex]
			} else {
				service.IP = containerIp
			}
			log.Debug("using container IP " + service.IP + " from label '" +
				b.config.UseIpFromLabel + "'")
		} else {
			log.Debug("Label '" + b.config.UseIpFromLabel +
				"' not found in container configuration")
		}
	}

	// NetworkMode can point to another container (kuberenetes pods)
	networkMode := container.HostConfig.NetworkMode
	if networkMode != "" {
		if strings.HasPrefix(networkMode, "container:") {
			networkContainerId := strings.Split(networkMode, ":")[1]
			log.Debug(service.Name + ": detected container NetworkMode, linked to: " + networkContainerId[:12])
			networkContainer, err := b.docker.InspectContainer(networkContainerId)
			if err != nil {
				log.Error("unable to inspect network container:", networkContainerId[:12], err)
			} else {
				service.IP = networkContainer.NetworkSettings.IPAddress
				log.Debug(service.Name + ": using network container IP " + service.IP)
			}
		}
	}

	if port.PortType == "udp" {
		service.Tags = combineTags(
			mapDefault(metadata, "tags", ""), b.config.ForceTags, "udp")
		service.ID = service.ID + ":udp"
	} else {
		service.Tags = combineTags(
			mapDefault(metadata, "tags", ""), b.config.ForceTags)
	}

	// Look for ECS labels and add them to metadata if present
	for k, v := range container.Config.Labels {
		if strings.Contains(k, "com.amazonaws.ecs") {
			metadata[k] = v
		}
	}

	id := mapDefault(metadata, "id", "")
	if id != "" {
		service.ID = id
	}

	delete(metadata, "id")
	delete(metadata, "tags")
	delete(metadata, "name")
	service.Attrs = metadata
	service.TTL = b.config.RefreshTtl

	return service
}

func (b *Bridge) remove(containerId string, deregister bool) {
	log.Debugf("container stop detected for: %v", containerId)
	b.Lock()
	defer b.Unlock()

	if deregister {
		deregisterAll := func(services []*Service) {
			for _, service := range services {
				err := b.registry.Deregister(service)
				if err != nil {
					log.Error("deregister failed:", service.ID, err)
					continue
				}
				log.Debug("removed:", containerId[:12], service.ID)
			}
		}
		deregisterAll(b.services[containerId])
		if d := b.deadContainers[containerId]; d != nil {
			deregisterAll(d.Services)
			delete(b.deadContainers, containerId)
		}
	} else if b.config.RefreshTtl != 0 && b.services[containerId] != nil {
		// need to stop the refreshing, but can't delete it yet
		b.deadContainers[containerId] = &DeadContainer{b.config.RefreshTtl, b.services[containerId]}
	}
	delete(b.services, containerId)
}

// bit set on ExitCode if it represents an exit via a signal
var dockerSignaledBit = 128

func (b *Bridge) shouldRemove(containerId string) bool {
	if b.config.DeregisterCheck == "always" {
		return true
	}
	container, err := b.docker.InspectContainer(containerId)
	if _, ok := err.(*dockerapi.NoSuchContainer); ok {
		// the container has already been removed from Docker
		// e.g. probabably run with "--rm" to remove immediately
		// so its exit code is not accessible
		log.Debugf("registrator: container %v was removed, could not fetch exit code", containerId[:12])
		return true
	}

	switch {
	case err != nil:
		log.Errorf("registrator: error fetching status for container %v on \"die\" event: %v\n", containerId[:12], err)
		return false
	case container.State.Running:
		log.Debugf("registrator: not removing container %v, still running", containerId[:12])
		return false
	case container.State.ExitCode == 0:
		return true
	case container.State.ExitCode&dockerSignaledBit == dockerSignaledBit:
		return true
	}
	return false
}

func (b *Bridge) getIpFromServer() string {
	client := http.Client{
		Timeout: Timeout,
	}
	resp, err := client.Get(b.config.IpServer)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	defer resp.Body.Close()
	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	return string(responseData)
}

var Hostname string

func init() {
	// It's ok for Hostname to ultimately be an empty string
	// An empty string will fall back to trying to make a best guess
	Hostname, _ = os.Hostname()
}
