package aws

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"reflect"
	"strconv"
	"time"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/elbv2"

	"sync"

	"github.com/gliderlabs/registrator/bridge"
	fargo "github.com/hudl/fargo"
	gocache "github.com/patrickmn/go-cache"
)

// LBInfo represents a ELBv2 endpoint
type LBInfo struct {
	DNSName        string
	Port           int64
	TargetGroupArn string
}

type lookupValues struct {
	InstanceID  string
	Port        int64
	ClusterName string
	TaskArn     string
}

const DEFAULT_EXP_TIME = 10 * time.Second

var generalCache = gocache.New(DEFAULT_EXP_TIME, DEFAULT_EXP_TIME)
var refreshInterval int

type any interface{}

// HasNoLoadBalancer - Special error type for when container has no load balancer
type HasNoLoadBalancer struct {
	message string
}

func (e HasNoLoadBalancer) Error() string {
	return e.message
}

//
// Make eureka status thread-safe
//
type eurekaStatus struct {
	sync.Mutex
	Mapper map[string]fargo.StatusType
}

var previousStatus = eurekaStatus{Mapper: make(map[string]fargo.StatusType)}

func getPreviousStatus(containerID string) fargo.StatusType {
	previousStatus.Lock()
	defer previousStatus.Unlock()
	return previousStatus.Mapper[containerID]
}

func setPreviousStatus(containerID string, status fargo.StatusType) {
	previousStatus.Lock()
	defer previousStatus.Unlock()
	previousStatus.Mapper[containerID] = status
}

//
// Provide a general caching mechanism for any function that lasts a few seconds.
//
func getAndCache(key string, input any, f any, cacheTime time.Duration) (any, error) {

	vf := reflect.ValueOf(f)
	vinput := reflect.ValueOf(input)

	result, found := generalCache.Get(key)
	if !found {
		caller := vf.Call([]reflect.Value{vinput})
		output := caller[0].Interface()
		err, _ := caller[1].Interface().(error)
		if err == nil {
			generalCache.Set(key, output, cacheTime)
			return output, nil
		}
		return nil, err
	}
	return result, nil
}

// RemoveLBCache : Delete any cache of load balancer for this containerID
func RemoveLBCache(key string) {
	generalCache.Delete(key)
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
	return ecs.New(sess, awssdk.NewConfig().WithRegion("us-east-1")), nil
}

// Get Load balancer and target group using a service and cluster name (more efficient)
func getLoadBalancerFromService(serviceName string, clusterName string) (*elbv2.LoadBalancer, *elbv2.TargetGroup, error) {

	dsi := ecs.DescribeServicesInput{
		Cluster:  &clusterName,
		Services: []*string{&serviceName},
	}

	svc, err := getECSSession()
	if err != nil {
		return nil, nil, err
	}
	svc2, err := getSession()
	if err != nil {
		return nil, nil, err
	}

	out, err := svc.DescribeServices(&dsi)
	if err != nil || out == nil {
		log.Printf("An error occurred using DescribeServices: %s \n", err.Error())
		return nil, nil, err
	}
	if len(out.Services) == 0 || len(out.Services[0].LoadBalancers) == 0 {
		hnb := HasNoLoadBalancer{message: "Load balancer not found.  It possibly doesn't exist for this service."}
		return nil, nil, hnb
	}
	tgArn := out.Services[0].LoadBalancers[0].TargetGroupArn

	// Get the target group listed for the service
	dtgI := elbv2.DescribeTargetGroupsInput{
		TargetGroupArns: []*string{tgArn},
	}
	out3, err := svc2.DescribeTargetGroups(&dtgI)
	if err != nil || out3 == nil {
		log.Printf("An error occurred using DescribeTargetGroups: %s \n", err.Error())
		return nil, nil, err
	}

	// Get the load balancer details
	dlbI := elbv2.DescribeLoadBalancersInput{
		LoadBalancerArns: out3.TargetGroups[0].LoadBalancerArns,
	}
	out2, err := svc2.DescribeLoadBalancers(&dlbI)
	if err != nil || out2 == nil {
		log.Printf("An error occurred using DescribeLoadBalancers: %s \n", err.Error())
		return nil, nil, err
	}

	return out2.LoadBalancers[0], out3.TargetGroups[0], nil
}

// Helper function to retrieve all target groups
func getAllTargetGroups(svc *elbv2.ELBV2) ([]*elbv2.DescribeTargetGroupsOutput, error) {
	var tgs []*elbv2.DescribeTargetGroupsOutput
	var e error
	var mark *string

	// Get first page of groups
	tg, e := getTargetGroupsPage(svc, mark)

	if e != nil {
		return nil, e
	}
	tgs = append(tgs, tg)
	mark = tg.NextMarker

	// Page through all remaining target groups generating a slice of DescribeTargetGroupOutputs
	for mark != nil {
		tg, e = getTargetGroupsPage(svc, mark)
		tgs = append(tgs, tg)
		mark = tg.NextMarker
		if e != nil {
			return nil, e
		}
	}
	return tgs, e
}

// Helper function to get a page of target groups
func getTargetGroupsPage(svc *elbv2.ELBV2, marker *string) (*elbv2.DescribeTargetGroupsOutput, error) {
	params := &elbv2.DescribeTargetGroupsInput{
		PageSize: awssdk.Int64(400),
		Marker:   marker,
	}

	tg, e := svc.DescribeTargetGroups(params)

	if e != nil {
		log.Printf("An error occurred using DescribeTargetGroups: %s \n", e.Error())
		return nil, e
	}
	return tg, nil
}

// GetELBV2ForContainer returns an LBInfo struct with the load balancer DNS name and listener port for a given instanceId and port
// if an error occurs, or the target is not found, an empty LBInfo is returned.
// Pass it the instanceID for the docker host, and the the host port to lookup the associated ELB.
//
func GetELBV2ForContainer(containerID string, instanceID string, port int64, clusterName string, taskArn string) (lbinfo *LBInfo, err error) {
	i := lookupValues{InstanceID: instanceID, Port: port, ClusterName: clusterName, TaskArn: taskArn}
	out, err := getAndCache(containerID, i, getLB, gocache.NoExpiration)
	if out == nil || err != nil {
		return nil, err
	}
	ret, _ := out.(*LBInfo)
	return ret, err
}

// GetHealthyTargets Get a list of healthy targets given a target group ARN.  Cache for default interval
func GetHealthyTargets(tgArn string) (thds []*elbv2.TargetHealthDescription, err error) {
	var healthCheckCacheTime time.Duration
	if refreshInterval != 0 {
		healthCheckCacheTime = (time.Duration(refreshInterval) * time.Second) - (1 * time.Second)
	} else {
		healthCheckCacheTime = DEFAULT_EXP_TIME
	}

	out, err := getAndCache(tgArn, tgArn, getHealthyTargets, healthCheckCacheTime)
	if out == nil || err != nil {
		return nil, err
	}
	ret, _ := out.([]*elbv2.TargetHealthDescription)
	return ret, err
}

// Actual func outside of caching mechanism
func getHealthyTargets(tgArn string) (ths []*elbv2.TargetHealthDescription, err error) {
	log.Printf("Looking for healthy targets")
	svc, err := getSession()
	if err != nil {
		return nil, err
	}

	thParams := &elbv2.DescribeTargetHealthInput{
		TargetGroupArn: awssdk.String(tgArn),
	}

	tarH, err := svc.DescribeTargetHealth(thParams)
	if err != nil || tarH == nil {
		log.Printf("An error occurred using DescribeTargetHealth: %s \n", err.Error())
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

// Lookup the service name from the ECS cluster and taskArn.  A bit janky, but works.
// Would be nice if amazon just put the service name as a label on the container.
func lookupServiceName(clusterName string, taskArn string) string {

	log.Printf("Looking up service with cluster: %s and taskArn: %s", clusterName, taskArn)
	svc, err := getECSSession()
	if err != nil {
		log.Printf("Unable to get ECS session: %s", err)
		return ""
	}

	dti := ecs.DescribeTasksInput{
		Cluster: &clusterName,
		Tasks:   []*string{&taskArn},
	}

	lsi := ecs.ListServicesInput{
		Cluster: &clusterName,
	}

	var servicesList []*string
	err = svc.ListServicesPages(&lsi,
		func(page *ecs.ListServicesOutput, lastPage bool) bool {
			for _, s := range page.ServiceArns {
				servicesList = append(servicesList, s)
			}
			return !lastPage
		})
	if err != nil || servicesList == nil {
		log.Printf("Error occurred using ListServicesPages: %s", err)
		return ""
	}

	dtout, err := svc.DescribeTasks(&dti)
	if err != nil || dtout == nil || len(dtout.Tasks) == 0 {
		log.Printf("Error occurred using DescribeTasks: %s", err)
		return ""
	}
	taskDefArn := dtout.Tasks[0].TaskDefinitionArn
	log.Printf("Task definition is: %v", *taskDefArn)

	// Lookup using maximum chunks of 10 service names
	var servChunk []*string
	for i := 0; i < len(servicesList); i++ {

		servChunk = append(servChunk, servicesList[i])
		// Make an API call every 10 service names, or on the last iteration over servicesList
		if (math.Mod(float64(i+1), 10) == 0 && i > 0) || i == (len(servicesList)-1) {
			dsi := ecs.DescribeServicesInput{
				Cluster:  &clusterName,
				Services: servChunk,
			}

			dsout, err := svc.DescribeServices(&dsi)
			if err != nil || dsout == nil {
				log.Printf("Error occurred using DescribeServices: %s", err)
				return ""
			}
			for _, ser := range dsout.Services {
				if *ser.TaskDefinition == *taskDefArn {
					return *ser.ServiceName
				}
			}
			servChunk = nil
		}
	}
	log.Printf("Service could not be identified")
	return ""
}

//
// Does the real work of retrieving the load balancer details, given a lookupValues struct.
// Note: This function uses caching extensively to reduce the burden on the AWS API when called from multiple goroutines
//
func getLB(l lookupValues) (lbinfo *LBInfo, err error) {
	instanceID := l.InstanceID
	port := l.Port

	var lbArns []*string
	var lbPort *int64
	var tgArn string
	info := &LBInfo{}
	var clusterName string
	var serviceName string

	// Small random wait to reduce risk of throttling
	seed := rand.NewSource(time.Now().UnixNano())
	r2 := rand.New(seed)
	random := r2.Intn(5000)
	period := time.Millisecond * time.Duration(random)
	log.Printf("Waiting for %v on ELBv2 lookup to avoid throttling.", period)
	time.Sleep(period)

	svc, err := getSession()
	if err != nil {
		return nil, err
	}

	// We've got a clusterName and taskArn so we can lookup the service
	if l.ClusterName != "" && l.TaskArn != "" {
		serviceName = lookupServiceName(l.ClusterName, l.TaskArn)
		clusterName = l.ClusterName
	}

	var tgslice []*elbv2.DescribeTargetGroupsOutput
	if serviceName == "" {
		// There could be thousands of these, and we need to check them all.
		// much better to have a service name to use.

		out1, err := getAndCache("tg", svc, getAllTargetGroups, DEFAULT_EXP_TIME)
		if err != nil || out1 == nil {
			message := fmt.Errorf("Failed to retrieve Target Groups: %s", err)
			return nil, message
		}
		tgslice, _ = out1.([]*elbv2.DescribeTargetGroupsOutput)

		// Check each target group's target list for a matching port and instanceID
		// Assumption: that that there is only one LB for the target group (though the data structure allows more)
		for _, tgs := range tgslice {
			log.Printf("%v target groups to check.", len(tgs.TargetGroups))
			for _, tg := range tgs.TargetGroups {

				thParams := &elbv2.DescribeTargetHealthInput{
					TargetGroupArn: awssdk.String(*tg.TargetGroupArn),
				}

				out2, err := getAndCache(*thParams.TargetGroupArn, thParams, svc.DescribeTargetHealth, DEFAULT_EXP_TIME)
				if err != nil || out2 == nil {
					log.Printf("An error occurred using DescribeTargetHealth: %s \n", err.Error())
					return nil, err
				}
				tarH, ok := out2.(*elbv2.DescribeTargetHealthOutput)
				if !ok || tarH.TargetHealthDescriptions == nil {
					continue
				}
				for _, thd := range tarH.TargetHealthDescriptions {
					if *thd.Target.Port == port && *thd.Target.Id == instanceID {
						log.Printf("Target group matched - %v", *tg.TargetGroupArn)
						lbArns = tg.LoadBalancerArns
						tgArn = *tg.TargetGroupArn
						break
					}
				}
			}
			if lbArns != nil && tgArn != "" {
				break
			}
		}

		if err != nil || lbArns == nil {
			message := fmt.Errorf("failed to retrieve load balancer ARN")
			return nil, message
		}

	} else {
		// We have the service and cluster name to use
		lb, tg, err := getLoadBalancerFromService(serviceName, clusterName)
		if lb == nil || tg == nil || err != nil {
			return nil, err
		}
		tgArn = *tg.TargetGroupArn
		lbArns = []*string{lb.LoadBalancerArn}
	}

	// Loop through the load balancer listeners to get the listener port for the target group
	lsnrParams := &elbv2.DescribeListenersInput{
		LoadBalancerArn: lbArns[0],
	}
	out3, err := getAndCache("lsnr_"+*lsnrParams.LoadBalancerArn, lsnrParams, svc.DescribeListeners, DEFAULT_EXP_TIME)
	if err != nil || out3 == nil {
		log.Printf("An error occurred using DescribeListeners: %s \n", err.Error())
		return nil, err
	}
	lnrData, _ := out3.(*elbv2.DescribeListenersOutput)
	for _, listener := range lnrData.Listeners {
		for _, act := range listener.DefaultActions {
			if *act.TargetGroupArn == tgArn {
				log.Printf("Found matching listener: %v", *listener.ListenerArn)
				lbPort = listener.Port
				break
			}
		}
	}
	if lbPort == nil {
		message := fmt.Errorf("error: Unable to identify listener port for ELBv2")
		return nil, message
	}

	// Get more information on the load balancer to retrieve the DNSName
	lbParams := &elbv2.DescribeLoadBalancersInput{
		LoadBalancerArns: lbArns,
	}
	out4, err := getAndCache("lb_"+*lbParams.LoadBalancerArns[0], lbParams, svc.DescribeLoadBalancers, DEFAULT_EXP_TIME)
	if err != nil || out4 == nil {
		log.Printf("An error occurred using DescribeLoadBalancers: %s \n", err.Error())
		return nil, err
	}
	lbData, _ := out4.(*elbv2.DescribeLoadBalancersOutput)
	log.Printf("LB Endpoint for Instance:%v Port:%v, Target Group:%v, is: %s:%s\n", instanceID, port, tgArn, *lbData.LoadBalancers[0].DNSName, strconv.FormatInt(*lbPort, 10))

	info.DNSName = *lbData.LoadBalancers[0].DNSName
	info.Port = *lbPort
	info.TargetGroupArn = tgArn
	return info, nil
}

// CheckELBFlags - Helper function to check if the correct config flags are set to use ELBs
// We accept two possible configurations here - either eureka_lookup_elbv2_endpoint can be set,
// for automatic lookup, or eureka_elbv2_hostname, eureka_elbv2_port and eureka_elbv2_targetgroup can be set manually
// to avoid the 10-20s wait for lookups
func CheckELBFlags(service *bridge.Service) bool {

	isAws := service.Attrs["eureka_datacenterinfo_name"] != fargo.MyOwn
	var hasExplicit bool
	var useLookup bool

	if service.Attrs["eureka_elbv2_hostname"] != "" && service.Attrs["eureka_elbv2_port"] != "" && service.Attrs["eureka_elbv2_targetgroup"] != "" {
		v, err := strconv.ParseUint(service.Attrs["eureka_elbv2_port"], 10, 16)
		if err != nil {
			log.Printf("eureka_elbv2_port must be valid 16-bit unsigned int, was %v : %s", v, err)
			hasExplicit = false
		}
		hasExplicit = true
		useLookup = true
	}

	if service.Attrs["eureka_lookup_elbv2_endpoint"] != "" {
		v, err := strconv.ParseBool(service.Attrs["eureka_lookup_elbv2_endpoint"])
		if err != nil {
			log.Printf("eureka_lookup_elbv2_endpoint must be valid boolean, was %v : %s", v, err)
			useLookup = false
		}
		useLookup = v
	}

	if (hasExplicit || useLookup) && isAws {
		return true
	}
	return false
}

// CheckELBOnlyReg - Helper function to check if only the ELB should be registered (no containers)
func CheckELBOnlyReg(service *bridge.Service) bool {

	if service.Attrs["eureka_elbv2_only_registration"] != "" {
		v, err := strconv.ParseBool(service.Attrs["eureka_elbv2_only_registration"])
		if err != nil {
			log.Printf("eureka_elbv2_only_registration must be valid boolean, was %v : %s", v, err)
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
func setRegInfo(service *bridge.Service, registration *fargo.Instance) *fargo.Instance {

	awsMetadata := GetMetadata()
	var elbEndpoint string
	var elbMetadata LBInfo

	// We've been given the ELB endpoint, so use this
	if service.Attrs["eureka_elbv2_hostname"] != "" && service.Attrs["eureka_elbv2_port"] != "" && service.Attrs["eureka_elbv2_targetgroup"] != "" {
		log.Printf("found ELBv2 hostname=%v, port=%v and TG=%v options, using these.", service.Attrs["eureka_elbv2_hostname"], service.Attrs["eureka_elbv2_port"], service.Attrs["eureka_elbv2_targetgroup"])
		registration.Port, _ = strconv.Atoi(service.Attrs["eureka_elbv2_port"])
		registration.HostName = service.Attrs["eureka_elbv2_hostname"]
		registration.IPAddr = ""
		registration.VipAddress = ""
		elbEndpoint = service.Attrs["eureka_elbv2_hostname"] + "_" + service.Attrs["eureka_elbv2_port"]
		elbMetadata = LBInfo{
			Port:           int64(registration.Port),
			DNSName:        registration.HostName,
			TargetGroupArn: service.Attrs["eureka_elbv2_targetgroup"],
		}

	} else {
		// We don't have the ELB endpoint, so look it up.
		// Check for some ECS labels first, these will allow more efficient lookups
		var clusterName string
		var taskArn string

		if service.Attrs["com.amazonaws.ecs.cluster"] != "" {
			clusterName = service.Attrs["com.amazonaws.ecs.cluster"]
		}
		if service.Attrs["com.amazonaws.ecs.task-arn"] != "" {
			taskArn = service.Attrs["com.amazonaws.ecs.task-arn"]
		}

		elbMetadata1, err := GetELBV2ForContainer(service.Origin.ContainerID, awsMetadata.InstanceID, int64(registration.Port), clusterName, taskArn)
		if err != nil || elbMetadata1 == nil {
			log.Printf("Unable to find associated ELBv2 for service: %s, instance: %s hostname: %s port: %v, Error: %s\n", service.Name, awsMetadata.InstanceID, registration.HostName, registration.Port, err)
			return nil
		}
		elbMetadata = *elbMetadata1

		elbStrPort := strconv.FormatInt(elbMetadata.Port, 10)
		elbEndpoint = elbMetadata.DNSName + "_" + elbStrPort
		registration.Port = int(elbMetadata.Port)
		registration.IPAddr = ""
		registration.HostName = elbMetadata.DNSName
	}

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
	registration.SetMetadataString("has-elbv2", "true")
	registration.SetMetadataString("elbv2-endpoint", elbEndpoint)
	registration.VipAddress = registration.IPAddr
	return registration
}

// Check an ELB's initial status in eureka
func getELBStatus(client fargo.EurekaConnection, registration *fargo.Instance) fargo.StatusType {
	result, err := client.GetInstance(registration.App, GetUniqueID(*registration))
	if err != nil || result == nil {
		log.Printf("ELB not yet present, or error retrieving from eureka: %s\n", err)
		return fargo.UNKNOWN
	}
	return result.Status
}

// Return appropriate registration statuses based on input and cached ELB data
func testStatus(containerID string, eurekaStatus fargo.StatusType, inputStatus fargo.StatusType) (newStatus fargo.StatusType, regStatus fargo.StatusType) {

	// Nothing to do if eureka says we're up, just return UP
	if eurekaStatus == fargo.UP {
		return fargo.UP, fargo.UP
	}

	if inputStatus != fargo.UP {
		log.Printf("Previous status was: %v, need to check for healthy targets.", inputStatus)
		// The ELB data should be cached, so just get it from there.
		result, found := generalCache.Get(containerID)
		if !found {
			log.Printf("Unable to retrieve ELB data from cache.  Cannot check for healthy targets!")
		}
		elbMetadata, ok := result.(*LBInfo)
		if !ok {
			log.Printf("Unable to convert LBInfo from cache.  Cannot check for healthy targets!")
		}
		log.Printf("Looking up healthy targets for TG: %v", elbMetadata.TargetGroupArn)
		thList, err2 := GetHealthyTargets(elbMetadata.TargetGroupArn)
		if err2 != nil {
			log.Printf("An error occurred looking up healthy targets, for target group: %s, will set to STARTING in eureka. Error: %s\n", elbMetadata.TargetGroupArn, err2)
			return fargo.UNKNOWN, fargo.STARTING
		}
		if len(thList) == 0 {
			log.Printf("Waiting on a healthy target in TG: %s - currently all UNHEALTHY.  Setting eureka state to STARTING.  This is normal for a new service which is starting up.  It may indicate a problem otherwise.", elbMetadata.TargetGroupArn)
			return fargo.STARTING, fargo.STARTING
		}
		log.Printf("Found %v healthy targets for target group: %s.  Setting eureka state to UP.", len(thList), elbMetadata.TargetGroupArn)
		return fargo.UP, fargo.UP
	}
	return fargo.UP, fargo.UP
}

// Test eureka registration status and mutate registration accordingly depending on container health.
func testHealth(service *bridge.Service, client fargo.EurekaConnection, elbReg *fargo.Instance) {
	containerID := service.Origin.ContainerID

	// Get actual eureka status and lookup previous logical registration status
	eurekaStatus := getELBStatus(client, elbReg)
	log.Printf("Eureka status check gave: %v", eurekaStatus)
	last := getPreviousStatus(containerID)

	// Work out an appropriate registration status given previous and current values
	newStatus, regStatus := testStatus(containerID, eurekaStatus, last)
	setPreviousStatus(containerID, newStatus)
	elbReg.Status = regStatus
	log.Printf("Status health check returned prev: %v registration: %v", last, elbReg.Status)
}

// RegisterWithELBv2 - If called, and flags are active, register an ELBv2 endpoint instead of the container directly
// This will mean traffic is directed to the ALB rather than directly to containers
func RegisterWithELBv2(service *bridge.Service, registration *fargo.Instance, client fargo.EurekaConnection) error {
	if CheckELBFlags(service) {
		log.Printf("Found ELBv2 flags, will attempt to register LB for: %s\n", GetUniqueID(*registration))
		elbReg := setRegInfo(service, registration)
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
			log.Printf("Retrying retrieval of ELBv2 data, attempt %v/3 - Waiting for %v seconds", i, period)
			time.Sleep(period)
			elbReg = setRegInfo(service, registration)
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
		log.Printf("Heartbeating ELBv2: %s\n", GetUniqueID(*registration))
		elbReg := setRegInfo(service, registration)
		if elbReg != nil {
			err := client.HeartBeatInstance(elbReg)
			// If the status of the ELB has not established as up, then recheck the health
			if getPreviousStatus(service.Origin.ContainerID) != fargo.UP {
				testHealth(service, client, elbReg)
				err := client.ReregisterInstance(elbReg)
				if err != nil {
					log.Printf("An error occurred when attempting to reregister ELB: %s", err)
					return err
				}
			}
			return err
		}
	}
	return fmt.Errorf("unable to heartbeat ELBv2. %s", GetUniqueID(*registration))
}
