package bridge

import (
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	dockerapi "github.com/fsouza/go-dockerclient"
)

var ipLookupAddress = ""
var ipLookupRetries = 0
var ipRetryInterval = 10
var client = http.Client{
	Timeout: time.Duration(60 * time.Second),
}

func retry(fn func() error) error {
	return backoff.Retry(fn, backoff.NewExponentialBackOff())
}

func mapDefault(m map[string]string, key, default_ string) string {
	v, ok := m[key]
	if !ok || v == "" {
		return default_
	}
	return v
}

func SetExternalIPSource(lookupAddress string) {
	ipLookupAddress = lookupAddress
}

func SetIPLookupRetries(number int) {
	ipLookupRetries = number
}

func lookupIp(address string) (*http.Response, error) {
	return client.Get(address)
}

func GetIPFromExternalSource() (string, bool) {
	_ip := []byte{}
	attempt := 1
	for attempt <= ipLookupRetries {
		res, err := lookupIp(ipLookupAddress)
		if err != nil {
			log.Errorf("Failed to lookup IP Address from external source: %s. Waiting before attempting retry...", ipLookupAddress, err)
			select {
			case <-time.After(time.Duration(ipRetryInterval*attempt) * time.Second):
				attempt++
				continue
			}
		}

		ip, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Error("Failed to read body of lookup from external source. Attempting retry", err)

			select {
			case <-time.After(time.Duration(ipRetryInterval*attempt) * time.Second):
				attempt++
				continue
			}
		}

		_ip = ip
		break
	}

	if len(_ip) == 0 {
		log.Error("All retries used when getting ip from external source.")
		return "", false
	}
	ipValue := string(_ip)
	log.Infof("Success, returning ip: %s", ipValue)
	return ipValue, true
}

// Golang regexp module does not support /(?!\\),/ syntax for spliting by not escaped comma
// Then this function is reproducing it
func recParseEscapedComma(str string) []string {
	if len(str) == 0 {
		return []string{}
	} else if str[0] == ',' {
		return recParseEscapedComma(str[1:])
	}

	offset := 0
	for len(str[offset:]) > 0 {
		index := strings.Index(str[offset:], ",")

		if index == -1 {
			break
		} else if str[offset+index-1:offset+index] != "\\" {
			return append(recParseEscapedComma(str[offset+index+1:]), str[:offset+index])
		}

		str = str[:offset+index-1] + str[offset+index:]
		offset += index
	}

	return []string{str}
}

func combineTags(tagParts ...string) []string {
	tags := make([]string, 0)
	for _, element := range tagParts {
		tags = append(tags, recParseEscapedComma(element)...)
	}
	return tags
}

func lookupMetaData(config *dockerapi.Config, key string) string {
	for _, v := range config.Env {
		str := strings.SplitN(v, "=", 2)
		if strings.EqualFold(str[0], key) {
			return str[1]
		}
	}
	return ""
}

func serviceMetaData(config *dockerapi.Config, port string) (map[string]string, map[string]bool) {
	meta := config.Labels

	// Env take precedence over labels
	for _, v := range config.Env {
		str := strings.SplitN(v, "=", 2)
		meta[str[0]] = str[1]
	}
	metadata := make(map[string]string)
	metadataFromPort := make(map[string]bool)
	for ks, kv := range meta {
		if strings.HasPrefix(ks, "SERVICE_") && len(ks) > 1 {
			key := strings.ToLower(strings.TrimPrefix(ks, "SERVICE_"))
			if metadataFromPort[key] {
				continue
			}
			portkey := strings.SplitN(key, "_", 2)
			_, err := strconv.Atoi(portkey[0])
			if err == nil && len(portkey) > 1 {
				if portkey[0] != port {
					continue
				}
				metadata[portkey[1]] = kv
				metadataFromPort[portkey[1]] = true
			} else {
				metadata[key] = kv
			}
		}
	}
	return metadata, metadataFromPort
}

func servicePort(container *dockerapi.Container, port dockerapi.Port, published []dockerapi.PortBinding) ServicePort {
	var hp, hip, ep, ept, eip, nm string
	if len(published) > 0 {
		hp = published[0].HostPort
		hip = published[0].HostIP
	}
	if hip == "" {
		hip = "0.0.0.0"
	}

	//for overlay networks
	//detect if container use overlay network, than set HostIP into NetworkSettings.Network[string].IPAddress
	//better to use registrator with -internal flag
	nm = container.HostConfig.NetworkMode
	if nm != "bridge" && nm != "default" && nm != "host" {
		hip = container.NetworkSettings.Networks[nm].IPAddress
	}

	exposedPort := strings.Split(string(port), "/")
	ep = exposedPort[0]
	if len(exposedPort) == 2 {
		ept = exposedPort[1]
	} else {
		ept = "tcp" // default
	}

	// Nir: support docker NetworkSettings
	eip = container.NetworkSettings.IPAddress
	if eip == "" {
		for _, network := range container.NetworkSettings.Networks {
			eip = network.IPAddress
		}
	}

	return ServicePort{
		HostPort:          hp,
		HostIP:            hip,
		ExposedPort:       ep,
		ExposedIP:         eip,
		PortType:          ept,
		ContainerID:       container.ID,
		ContainerName:     container.Name,
		ContainerHostname: container.Config.Hostname,
		container:         container,
	}
}

// Used to sync all services
func serviceSync(b *Bridge, quiet bool, newIP string) {

	containers, err := b.docker.ListContainers(dockerapi.ListContainersOptions{})
	if err != nil && quiet {
		log.Error("error listing containers, skipping sync")
		return
	} else if err != nil && !quiet {
		log.Fatal(err)
	}

	log.Debugf("Syncing services on %d containers", len(containers))
	if newIP != "" {
		if b.config.HostIp != newIP {
			log.Infof("Bridge Config HostIP is different to new IP, adjusting: %s", newIP)
		}
	}
	// NOTE: This assumes reregistering will do the right thing, i.e. nothing..
	for _, listing := range containers {

		services := b.services[listing.ID]
		if services == nil {
			log.Debugf("Services are nil, building new services against listing: %s", listing.ID)
			go b.add(listing.ID, quiet, newIP)
		} else {
			for _, service := range services {
				log.Debugf("Service: %s", service)
				if newIP != "" {
					service.RLock()
					if service.IP != newIP {
						log.Info("Service has IP difference, reallocating: ", service.Name)
					} else {
						log.Info("Service already on correct IP: ", service.Name)
						service.RUnlock()
						continue
					}
					err := b.registry.Deregister(service)
					if err != nil {
						log.Error("Deregister during new IP Allocation failed:", service, err)
						service.RUnlock()
						continue
					}
					service.RUnlock()

					service.Lock()
					service.IP = newIP
					service.Origin.HostIP = newIP
					err = b.registry.Register(service)
					service.Unlock()

					if err != nil {
						log.Error("Register during new IP Allocation failed:", service, err)
						continue
					}
				}
				service.Lock()
				err := b.registry.Register(service)
				if err != nil {
					log.Debug("sync register failed:", service, err)
				}
				service.Unlock()
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
		for listingId, _ := range b.services {
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
			for _, listing := range b.services {
				for _, service := range listing {
					service.RLock()
					if service.Name == extService.Name && serviceContainerName == service.Origin.container.Name[1:] {
						service.RUnlock()
						continue Outer
					}
					service.RUnlock()
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
