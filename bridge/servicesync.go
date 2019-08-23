package bridge

import (
	"encoding/json"
	"sync"

	dockerapi "github.com/fsouza/go-dockerclient"
)

var SyncChannel = make(map[*Bridge]chan SyncMessage)
var filters = map[string][]string{"status": {"created", "restarting", "running", "paused"}}
var wg sync.WaitGroup

func Initialize(bridge *Bridge) {
	log.Info("Initialized Sync serivce channel")
	SyncChannel[bridge] = make(chan SyncMessage)
	go channelRun(bridge)
}

func channelRun(bridge *Bridge) {
	for {
		wg.Wait()
		wg.Add(1)
		val, ok := <-SyncChannel[bridge]
		if ok == false {
			log.Error("Sync service channel has been closed")
			break
		}
		bridge.Lock()
		serviceSync(val, bridge)
		bridge.Unlock()
		wg.Done()
	}
}

func reregisterService(registry RegistryAdapter, service *Service, newIP string) {
	repr, _ := json.MarshalIndent(service, "", " ")
	log.Debugf("Service: %s", repr)
	if newIP != "" {
		service.RLock()
		if service.IP != newIP {
			log.Info("Service has IP difference, reallocating: ", service.Name)
		} else {
			log.Info("Service already on correct IP: ", service.Name)
			service.RUnlock()
			return
		}
		err := registry.Deregister(service)
		if err != nil {
			log.Error("Deregister during new IP Allocation failed:", service, err)
			service.RUnlock()
			return
		}
		service.RUnlock()

		service.Lock()
		service.IP = newIP
		service.Origin.HostIP = newIP
		err = registry.Register(service)
		service.Unlock()

		if err != nil {
			log.Error("Register during new IP Allocation failed:", service, err)
			return
		}
		return
	}
	service.Lock()
	err := registry.Register(service)
	if err != nil {
		log.Debug("sync register failed:", service, err)
	}
	service.Unlock()
	return
}

func cleanupServices(b *Bridge, danglingServices []*Service) {
Outer:
	for _, extService := range danglingServices {
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

// Used to sync all services
func serviceSync(message SyncMessage, b *Bridge) {
	quiet := message.Quiet
	newIP := message.IP

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
				reregisterService(b.registry, service, newIP)
			}
		}
	}

	// Clean up services that were registered previously, but aren't
	// acknowledged within registrator
	if b.config.Cleanup {
		// Remove services if its corresponding container is not running
		log.Debug("Listing non-exited containers")
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

		cleanupServices(b, extServices)
	}
}
