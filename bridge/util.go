package bridge

import (
	"strconv"
	"strings"

	"github.com/cenkalti/backoff"
	containertypes "github.com/docker/docker/api/types/container"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"
)

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

func serviceMetaData(config *containertypes.Config, port string) (map[string]string, map[string]bool) {
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

func servicePort(container *dockertypes.ContainerJSON, port nat.Port, published []nat.PortBinding) ServicePort {
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
	nm = string(container.HostConfig.NetworkMode)
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
