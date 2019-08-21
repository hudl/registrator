package aws

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/elbv2"

	"github.com/gliderlabs/registrator/bridge"
	"github.com/hudl/fargo"
	gocache "github.com/patrickmn/go-cache"
)

var refreshInterval int

func (e HasNoLoadBalancer) Error() string {
	return e.message
}

// Get a session to AWS API
func getSession() (*elbv2.ELBV2, error) {
	sess, err := session.NewSession()
	if err != nil {
		message := fmt.Errorf("Failed to create session connecting to AWS: %s", err)
		return nil, message
	}

	// Need to set the region here - we'll get it from instance metadata
	awsMetadata := GetMetadata()
	return elbv2.New(sess, awssdk.NewConfig().WithRegion(awsMetadata.Region)), nil
}

func getECSSession() (*ecs.ECS, error) {
	sess, err := session.NewSession()
	if err != nil {
		message := fmt.Errorf("Failed to create session connecting to AWS: %s", err)
		return nil, message
	}

	// Need to set the region here - we'll get it from instance metadata
	awsMetadata := GetMetadata()
	return ecs.New(sess, awssdk.NewConfig().WithRegion(awsMetadata.Region)), nil
}

// CheckELBFlags - Helper function to check if the correct config flags are set to use ELBs
// eureka_elbv2_hostname, eureka_elbv2_port and eureka_elbv2_targetgroup must be set manually
func CheckELBFlags(service *bridge.Service) bool {

	isAws := service.Attrs["eureka_datacenterinfo_name"] != fargo.MyOwn
	var hasFlags bool

	if service.Attrs["eureka_elbv2_hostname"] != "" && service.Attrs["eureka_elbv2_port"] != "" && service.Attrs["eureka_elbv2_targetgroup"] != "" {
		v, err := strconv.ParseUint(service.Attrs["eureka_elbv2_port"], 10, 16)
		if err != nil {
			log.Errorf("eureka_elbv2_port must be valid 16-bit unsigned int, was %v : %s", v, err)
			hasFlags = false
		}
		hasFlags = true
	}

	if hasFlags && isAws {
		return true
	}
	return false
}

// CheckELBOnlyReg - Helper function to check if only the ELB should be registered (no containers)
func CheckELBOnlyReg(service *bridge.Service) bool {

	if service.Attrs["eureka_elbv2_only_registration"] != "" {
		v, err := strconv.ParseBool(service.Attrs["eureka_elbv2_only_registration"])
		if err != nil {
			log.Errorf("eureka_elbv2_only_registration must be valid boolean, was %v : %s", v, err)
			return true
		}
		return v
	}
	return true
}

// GetUniqueID Note: Helper function reimplemented here to avoid segfault calling it on fargo.Instance struct
func GetUniqueID(instance fargo.Instance) string {
	return instance.HostName + "_" + strconv.Itoa(instance.Port)
}

// Helper function to alter registration info and add the ELBv2 endpoint
// useCache parameter is passed to getELBV2ForContainer
func mutateRegistrationInfo(service *bridge.Service, registration *fargo.Instance) *fargo.Instance {

	elbMetadata, err := getELBMetadata(service, registration.HostName, registration.Port)
	if err != nil {
		return nil
	}

	registration.IPAddr = elbMetadata.IpAddress
	registration.VipAddress = elbMetadata.VipAddress
	registration.Port = elbMetadata.Port
	registration.HostName = elbMetadata.DNSName

	registration.SetMetadataString("has-elbv2", "true")
	registration.SetMetadataString("elbv2-endpoint", elbMetadata.ELBEndpoint)
	registration.VipAddress = registration.IPAddr

	if CheckELBOnlyReg(service) {
		// Remove irrelevant metadata from an ELB only registration
		registration.DataCenterInfo.Metadata = fargo.AmazonMetadataType{
			InstanceID:     GetUniqueID(*registration), // This is deliberate - due to limitations in uniqueIDs
			PublicHostname: registration.HostName,
			HostName:       registration.HostName,
		}
		registration.SetMetadataString("container-id", "")
		registration.SetMetadataString("container-name", "")
		registration.SetMetadataString("aws-instance-id", "")
	}

	// Reduce lease time for ALBs to have them drop out of eureka quicker
	registration.LeaseInfo = fargo.LeaseInfo{
		DurationInSecs: 35,
	}

	return registration
}

func getELBMetadata(service *bridge.Service, hostName string, port int) (LoadBalancerRegistrationInfo, error) {
	var elbMetadata LoadBalancerRegistrationInfo

	// We've been given the ELB endpoint, so use this
	if service.Attrs["eureka_elbv2_hostname"] != "" && service.Attrs["eureka_elbv2_port"] != "" && service.Attrs["eureka_elbv2_targetgroup"] != "" {
		log.Debugf("found ELBv2 hostname=%v, port=%v and TG=%v options, using these.", service.Attrs["eureka_elbv2_hostname"], service.Attrs["eureka_elbv2_port"], service.Attrs["eureka_elbv2_targetgroup"])
		elbMetadata.Port, _ = strconv.Atoi(service.Attrs["eureka_elbv2_port"])
		elbMetadata.DNSName = service.Attrs["eureka_elbv2_hostname"]
		elbMetadata.TargetGroupArn = service.Attrs["eureka_elbv2_targetgroup"]
		elbMetadata.ELBEndpoint = service.Attrs["eureka_elbv2_hostname"] + "_" + service.Attrs["eureka_elbv2_port"]
		elbMetadata.IpAddress = ""
		AddToCache("container_"+service.Origin.ContainerID, &elbMetadata, gocache.NoExpiration)
	}
	return elbMetadata, nil
}

// Check an ELB's initial status in eureka
func getELBStatus(client fargo.EurekaConnection, registration *fargo.Instance) fargo.StatusType {
	result, err := client.GetInstance(registration.App, GetUniqueID(*registration))
	if err != nil || result == nil {
		// Can't find the ELB, this is more than likely expected. It takes a short amount of time
		// after a container launch, for a new service, for the ELB to be fully provisioned.
		// This gets retried 3 times with the RegisterWithELBv2() method and an error is logged
		// after each of those fail.
		log.Warningf("ELB not yet present, or error retrieving from eureka: %s\n", err)
		return fargo.UNKNOWN
	}
	return result.Status
}

// RegisterWithELBv2 - If called, and flags are active, register an ELBv2 endpoint instead of the container directly
// This will mean traffic is directed to the ALB rather than directly to containers
func RegisterWithELBv2(service *bridge.Service, registration *fargo.Instance, client fargo.EurekaConnection) error {
	if CheckELBFlags(service) {
		log.Debugf("Found ELBv2 flags, will attempt to register load balancer for: %s\n", GetUniqueID(*registration))
		elbReg := mutateRegistrationInfo(service, registration)
		if elbReg != nil {
			testHealth(service, client, elbReg)
			err := client.ReregisterInstance(elbReg)
			return err
		}
		seed := rand.NewSource(time.Now().UnixNano())
		r2 := rand.New(seed)
		for i := 1; i < 4; i++ {
			// If there's no ELBv2 data, we need to retry a couple of times, as it takes a little while to propogate target group membership
			// To avoid any wait, the endpoints can be specified manually as eureka_elbv2_hostname and eureka_elbv2_port vars
			random := r2.Intn(5000)
			modifier := +time.Duration(time.Millisecond * 5000)
			period := time.Duration(time.Millisecond*time.Duration(random)) + modifier + time.Duration(DEFAULT_EXP_TIME*time.Duration(i))
			log.Debugf("Retrying retrieval of ELBv2 data, attempt %v/3 - Waiting for %v seconds", i, period)
			time.Sleep(period)
			elbReg = mutateRegistrationInfo(service, registration)
			if elbReg != nil {
				testHealth(service, client, elbReg)
				err := client.ReregisterInstance(elbReg)
				return err
			}
		}
	}
	return fmt.Errorf("unable to register ELBv2: %v", GetUniqueID(*registration))
}

// HeartbeatELBv2 - Heartbeat an ELB registration
func HeartbeatELBv2(service *bridge.Service, registration *fargo.Instance, client fargo.EurekaConnection) error {
	if CheckELBFlags(service) {
		log.Debugf("Heartbeating ELBv2: %s\n", GetUniqueID(*registration))
		elbReg := mutateRegistrationInfo(service, registration)
		if elbReg != nil {
			err := client.HeartBeatInstance(elbReg)
			// If the status of the ELB has not established as up, then recheck the health
			if getPreviousStatus(service.Origin.ContainerID) != fargo.UP {
				testHealth(service, client, elbReg)
				err := client.ReregisterInstance(elbReg)
				if err != nil {
					log.Errorf("An error occurred when attempting to reregister ELB: %s", err)
					return err
				}
			}
			return err
		}
	}
	return fmt.Errorf("unable to heartbeat ELBv2. %s", GetUniqueID(*registration))
}
