package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/gliderlabs/registrator/bridge"
	"github.com/hudl/fargo"
	"sync"
	"time"
)

type statusChange struct {
	registrationStatus fargo.StatusType
	newStatus          fargo.StatusType
}

type eurekaStatus struct {
	sync.RWMutex
	Mapper map[string]fargo.StatusType
}

var previousStatus = eurekaStatus{Mapper: make(map[string]fargo.StatusType)}

func getPreviousStatus(containerID string) fargo.StatusType {
	previousStatus.RLock()
	defer previousStatus.RUnlock()
	return previousStatus.Mapper[containerID]
}

func setPreviousStatus(containerID string, status fargo.StatusType) {
	previousStatus.Lock()
	defer previousStatus.Unlock()
	previousStatus.Mapper[containerID] = status
}

// GetHealthyTargets Get a list of healthy targets given a target group ARN.  Cache for default interval
func GetHealthyTargets(tgArn string) (thds []*elbv2.TargetHealthDescription, err error) {
	var healthCheckCacheTime time.Duration
	if refreshInterval != 0 {
		healthCheckCacheTime = (time.Duration(refreshInterval) * time.Second) - (1 * time.Second)
	} else {
		healthCheckCacheTime = DEFAULT_EXP_TIME
	}

	out, err := GetAndCache("tg_arn_" + tgArn, tgArn, getHealthyTargets, healthCheckCacheTime)
	if out == nil || err != nil {
		return nil, err
	}
	ret, _ := out.([]*elbv2.TargetHealthDescription)
	return ret, err
}

// Actual func outside of caching mechanism
func getHealthyTargets(tgArn string) (ths []*elbv2.TargetHealthDescription, err error) {
	log.Debugf("Looking for healthy targets")
	svc, err := getSession()
	if err != nil {
		return nil, err
	}

	thParams := &elbv2.DescribeTargetHealthInput{
		TargetGroupArn: aws.String(tgArn),
	}

	tarH, err := svc.DescribeTargetHealth(thParams)
	if err != nil || tarH == nil {
		log.Errorf("An error occurred using DescribeTargetHealth: %s \n", err.Error())
		return nil, err
	}

	var healthyTargets []*elbv2.TargetHealthDescription
	for _, thd := range tarH.TargetHealthDescriptions {
		if *thd.TargetHealth.State == "healthy" {
			healthyTargets = append(healthyTargets, thd)
		}
	}
	return healthyTargets, nil
}

// Test eureka registration status and mutate registration accordingly depending on container health.
func testHealth(service *bridge.Service, client fargo.EurekaConnection, elbReg *fargo.Instance) {
	containerID := service.Origin.ContainerID

	// Get actual eureka status and lookup previous logical registration status
	eurekaStatus := getELBStatus(client, elbReg)
	log.Debugf("Eureka status check gave: %v", eurekaStatus)
	last := getPreviousStatus(containerID)

	// Work out an appropriate registration status given previous and current values
	statusChange := determineNewEurekaStatus(containerID, eurekaStatus, last)
	setPreviousStatus(containerID, statusChange.newStatus)
	elbReg.Status = statusChange.registrationStatus
	log.Debugf("Status health check returned prev: %v registration: %v", last, elbReg.Status)
}

// Return appropriate registration statuses based on previous status and cached ELB data
func determineNewEurekaStatus(containerID string, eurekaStatus fargo.StatusType, inputStatus fargo.StatusType) (change statusChange) {

	// Nothing to do if eureka says we're up, just return UP
	if eurekaStatus == fargo.UP {
		return statusChange{newStatus: fargo.UP, registrationStatus: fargo.UP}
	}

	if inputStatus != fargo.UP {
		log.Debugf("Previous status was: %v, need to check for healthy targets.", inputStatus)
		// The ELB data should be cached, so just get it from there.
		result, found := generalCache.Get("container_" + containerID)
		if !found {
			log.Errorf("Unable to retrieve ELB data from cache.  Cannot check for healthy targets!")
			return statusChange{newStatus: fargo.UNKNOWN, registrationStatus: fargo.STARTING}
		}
		elbMetadata, ok := result.(*LoadBalancerRegistrationInfo)
		if !ok {
			log.Errorf("Unable to convert LoadBalancerRegistrationInfo from cache.  Cannot check for healthy targets!")
			return statusChange{newStatus: fargo.UNKNOWN, registrationStatus: fargo.STARTING}
		}
		log.Debugf("Looking up healthy targets for TG: %v", elbMetadata.TargetGroupArn)
		thList, err2 := GetHealthyTargets(elbMetadata.TargetGroupArn)
		if err2 != nil {
			log.Errorf("An error occurred looking up healthy targets, for target group: %s, will set to STARTING in eureka. Error: %s\n", elbMetadata.TargetGroupArn, err2)
			return statusChange{newStatus: fargo.UNKNOWN, registrationStatus: fargo.STARTING}
		}
		if len(thList) == 0 {
			log.Infof("Waiting on a healthy target in TG: %s - currently all UNHEALTHY.  Setting eureka state to STARTING.  This is normal for a new service which is starting up.  It may indicate a problem otherwise.", elbMetadata.TargetGroupArn)
			return statusChange{newStatus: fargo.STARTING, registrationStatus: fargo.STARTING}
		}
		log.Debugf("Found %v healthy targets for target group: %s.  Setting eureka state to UP.", len(thList), elbMetadata.TargetGroupArn)
		return statusChange{newStatus: fargo.UP, registrationStatus: fargo.UP}
	}
	return statusChange{newStatus: fargo.UP, registrationStatus: fargo.UP}
}
