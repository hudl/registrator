package bridge

import (
	"errors"
	"net"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"

	dockerapi "github.com/fsouza/go-dockerclient"
)

var serviceIDPattern = regexp.MustCompile(`^(.+?):([a-zA-Z0-9][a-zA-Z0-9_.-]+):[0-9]+(?::udp)?$`)

// Simple interface for testing
type DockerClient interface {
	InspectContainer(c string) (*dockerapi.Container, error)
	ListContainers(opts dockerapi.ListContainersOptions) ([]dockerapi.APIContainers, error)
}

type Bridge struct {
	sync.Mutex
	registry       RegistryAdapter
	docker         DockerClient
	services       map[string][]*Service
	deadContainers map[string]*DeadContainer
	config         Config
}

func New(docker DockerClient, adapterUri string, config Config) (*Bridge, error) {
	uri, err := url.Parse(adapterUri)
	if err != nil {
		return nil, errors.New("bad adapter uri: " + adapterUri)
	}
	factory, found := AdapterFactories.Lookup(uri.Scheme)
	if !found {
		return nil, errors.New("unrecognized adapter: " + adapterUri)
	}

	log.Debug("Using", uri.Scheme, "adapter:", uri)
	bridge := &Bridge{
		docker:         docker,
		config:         config,
		registry:       factory.New(uri),
		services:       make(map[string][]*Service),
		deadContainers: make(map[string]*DeadContainer),
	}

	Initialize(bridge)

	return bridge, nil
}

func (b *Bridge) Ping() error {
	return b.registry.Ping()
}

func (b *Bridge) Add(containerId, ipToUse string) {
	b.add(containerId, false, ipToUse)
}

func (b *Bridge) Remove(containerId string) {
	b.remove(containerId, true)
}

func (b *Bridge) RemoveOnExit(containerId string) {
	b.remove(containerId, b.shouldRemove(containerId))
}

func (b *Bridge) Refresh() {
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

func (b *Bridge) PushServiceSync(msg SyncMessage) {
	SyncChannel[b] <- msg
}

func (b *Bridge) Sync(quiet bool) {
	// serviceSync(b, quiet, "")
}

func (b *Bridge) AllocateNewIPToServices(ip string) {
	// serviceSync(b, true, ip)
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

func (b *Bridge) add(containerId string, quiet bool, newIP string) {
	log.Infof("Bridge.Add called with IP: %s", newIP)
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
		serviceP := servicePort(container, port, published)
		if newIP != "" {
			serviceP.HostIP = newIP
		}
		ports[string(port)] = serviceP
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
		// Added a check for the env var that sets up service.UseExposedPorts here and add the service ports if true
		log.Infof("looking up metadata for: %s", container.ID)
		useExposedPorts := lookupMetaData(container.Config, "SERVICE_USE_EXPOSED_PORTS")
		log.Infof("useExposedPorts: %s", useExposedPorts)

		if !b.config.Internal && useExposedPorts == "" && port.HostPort == "" {
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
	log.Info("Checking Ignore: %s", ignore)
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
	if service.Name == defaultName {
		log.Infof("Service does not have name via metadata. Default=%s, ContainerID=%s", defaultName, container.ID)
	} else {
		log.Infof("Service has metadata derived name=%s, Default=%s", service.Name, defaultName)
	}
	if strings.ToLower(mapDefault(metadata, "use_exposed_ports", "false")) == "true" {
		service.UseExposedPorts = true
	}

	if isgroup && !metadataFromPort["name"] {
		service.Name += "-" + port.ExposedPort
	}
	var convertedPort int

	log.Infof("New Service has config: Internal=%s UseExposedPorts=%s", b.config.Internal, service.UseExposedPorts)
	if b.config.Internal == true {
		service.IP = port.ExposedIP
	} else {
		service.IP = port.HostIP
	}
	if b.config.Internal == true || service.UseExposedPorts == true {
		p, err := strconv.Atoi(port.ExposedPort)
		if err != nil {
			log.Error("Unable to parse string ExposedPort to int: %s", port.ExposedPort)
		} else {
			convertedPort = p
		}
	} else {
		p, err := strconv.Atoi(port.HostPort)
		if err != nil {
			log.Error("Unable to parse string HostPort to int: %s", port.HostPort)
		} else {
			convertedPort = p
		}
	}
	service.Port = convertedPort

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

var Hostname string

func init() {
	// It's ok for Hostname to ultimately be an empty string
	// An empty string will fall back to trying to make a best guess
	Hostname, _ = os.Hostname()
}
