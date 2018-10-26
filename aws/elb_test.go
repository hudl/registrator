package aws

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/gliderlabs/registrator/bridge"
	eureka "github.com/hudl/fargo"
	gocache "github.com/patrickmn/go-cache"
)

// TestCheckELBOnlyReg - Test that ELBv2 only flag is evaulated correctly - default true
func TestCheckELBOnlyReg(t *testing.T) {

	svcFalse := bridge.Service{
		Attrs: map[string]string{
			"eureka_elbv2_only_registration": "false",
		},
	}

	svcTrue := bridge.Service{
		Attrs: map[string]string{
			"eureka_elbv2_only_registration": "true",
		},
	}

	svcTrue2 := bridge.Service{
		Attrs: map[string]string{},
	}

	type args struct {
		service *bridge.Service
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "set to false",
			args: args{service: &svcFalse},
			want: false,
		},
		{
			name: "set to true",
			args: args{service: &svcTrue},
			want: true,
		},
		{
			name: "not set",
			args: args{service: &svcTrue2},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckELBOnlyReg(tt.args.service); got != tt.want {
				t.Errorf("CheckELBOnlyReg() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Set up the cache
func setupCache(containerID string, instanceID string, lbDNSName string, containerPort int64, lbPort int, tgArn string, thds []*elbv2.TargetHealthDescription) {

	fn := func(l lookupValues) (*LoadBalancerRegistrationInfo, error) {
		return &LoadBalancerRegistrationInfo{DNSName: lbDNSName, Port: lbPort, TargetGroupArn: tgArn}, nil
	}
	i := lookupValues{InstanceID: instanceID, Port: containerPort}

	fn2 := func(thds []*elbv2.TargetHealthDescription) ([]*elbv2.TargetHealthDescription, error) {
		return thds, nil
	}
	GetAndCache("container_" + containerID, i, fn, gocache.NoExpiration)
	GetAndCache("tg_arn_" + tgArn, thds, fn2, gocache.NoExpiration)
	r, _ := generalCache.Get("container_" + containerID)
	fmt.Printf("Cache value now looks like this: %+v\n", r.(*LoadBalancerRegistrationInfo))
}


// Test_GetELBV2ForContainer - Test expected values are returned
func Test_GetELBV2ForContainer(t *testing.T) {

	// Setup cache
	lbWant := LoadBalancerRegistrationInfo{
		DNSName:        "my-lb",
		Port:           12345,
		TargetGroupArn: "arn:1234",
	}

	thds := []*elbv2.TargetHealthDescription{}
	tgArn := "arn:1234"
	setupCache("123123412", "instance-123", "my-lb", 1234, 12345, tgArn, thds)

	type args struct {
		containerID string
		instanceID  string
		cluster     string
		service     string
		task        string
		port        int64
	}
	tests := []struct {
		name       string
		args       args
		wantLbinfo *LoadBalancerRegistrationInfo
		wantErr    bool
	}{
		{
			name:       "should match",
			args:       args{containerID: "123123412", instanceID: "instance-123", port: int64(1234), cluster: "cluster", task: "task"},
			wantErr:    false,
			wantLbinfo: &lbWant,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLbinfo, err := GetELBV2ForContainer(tt.args.containerID, tt.args.instanceID, tt.args.port, tt.args.cluster, tt.args.task, tt.args.service)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetELBV2ForContainer() error = %+v, wantErr %+v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotLbinfo, tt.wantLbinfo) {
				t.Errorf("GetELBV2ForContainer() = %+v, want %+v", gotLbinfo, tt.wantLbinfo)
			}
		})
	}
}

// TestCheckELBFlags - Test that ELBv2 flags are evaulated correctly
func TestCheckELBFlags(t *testing.T) {

	svcFalse := bridge.Service{
		Attrs: map[string]string{
			"eureka_lookup_elbv2_endpoint": "false",
			"eureka_datacenterinfo_name":   "AMAZON",
		},
	}

	svcFalse2 := bridge.Service{
		Attrs: map[string]string{
			"eureka_lookup_elbv2_endpoint": "true",
			"eureka_datacenterinfo_name":   "MyOwn",
		},
	}

	svcFalse3 := bridge.Service{
		Attrs: map[string]string{
			"eureka_elbv2_hostname":      "my-name",
			"eureka_datacenterinfo_name": "AMAZON",
		},
	}

	svcFalse4 := bridge.Service{
		Attrs: map[string]string{
			"eureka_elbv2_hostname":      "my-name",
			"eureka_elbv2_port":          "1234",
			"eureka_datacenterinfo_name": "AMAZON",
		},
	}

	svcTrue := bridge.Service{
		Attrs: map[string]string{
			"eureka_lookup_elbv2_endpoint": "true",
			"eureka_datacenterinfo_name":   "AMAZON",
		},
	}

	svcTrue2 := bridge.Service{
		Attrs: map[string]string{
			"eureka_elbv2_hostname":      "my-name",
			"eureka_elbv2_port":          "1234",
			"eureka_elbv2_targetgroup":   "arn:1234",
			"eureka_datacenterinfo_name": "AMAZON",
		},
	}

	svcTrue3 := bridge.Service{
		Attrs: map[string]string{
			"eureka_elbv2_hostname":        "my-name",
			"eureka_lookup_elbv2_endpoint": "true",
			"eureka_elbv2_port":            "1234",
			"eureka_elbv2_targetgroup":     "arn:1234",
			"eureka_datacenterinfo_name":   "AMAZON",
		},
	}

	svcTrue4 := bridge.Service{
		Attrs: map[string]string{
			"eureka_elbv2_hostname":        "my-name",
			"eureka_lookup_elbv2_endpoint": "false",
			"eureka_elbv2_port":            "1234",
			"eureka_elbv2_targetgroup":     "arn:1234",
			"eureka_datacenterinfo_name":   "AMAZON",
		},
	}

	type args struct {
		service *bridge.Service
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "use lookup set to false",
			args: args{service: &svcFalse},
			want: false,
		},
		{
			name: "datacentre is set to MyOwn",
			args: args{service: &svcFalse2},
			want: false,
		},
		{
			name: "elb hostname, but not port is set",
			args: args{service: &svcFalse3},
			want: false,
		},
		{
			name: "elb hostname, port set, but no targetgroup",
			args: args{service: &svcFalse4},
			want: false,
		},
		{
			name: "elb lookup set to true",
			args: args{service: &svcTrue},
			want: true,
		},
		{
			name: "elb hostname and port are set",
			args: args{service: &svcTrue2},
			want: true,
		},
		{
			name: "elb hostname and port are set, as is lookup",
			args: args{service: &svcTrue3},
			want: true,
		},
		{
			name: "elb hostname, and port are set, lookup is false",
			args: args{service: &svcTrue4},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckELBFlags(tt.args.service); got != tt.want {
				t.Errorf("CheckELBFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test_mutateRegistrationInfo - Test that registration struct is returned as expected
func Test_mutateRegistrationInfo(t *testing.T) {
	initMetadata() // Used from metadata_test.go

	svc := bridge.Service{
		Attrs: map[string]string{
			"eureka_lookup_elbv2_endpoint": "true",
			"eureka_datacenterinfo_name":   "AMAZON",
		},
		Name: "app",
		Origin: bridge.ServicePort{
			ContainerID: "123123412",
		},
	}

	awsInfo := eureka.AmazonMetadataType{
		PublicHostname: "i-should-not-be-used",
		HostName:       "i-should-not-be-used",
		InstanceID:     "i-should-not-be-used",
	}

	dcInfo := eureka.DataCenterInfo{
		Name:     eureka.Amazon,
		Metadata: awsInfo,
	}

	reg := eureka.Instance{
		DataCenterInfo: dcInfo,
		Port:           5001,
		IPAddr:         "4.3.2.1",
		App:            "app",
		VipAddress:     "4.3.2.1",
		HostName:       "hostname_identifier",
		Status:         eureka.UP,
	}

	thds := []*elbv2.TargetHealthDescription{}
	tgArn := "arn:1234"
	// Init LB info cache
	setupCache("123123412", "instance-123", "correct-lb-dnsname", 1234, 9001, tgArn, thds)

	r, _ := generalCache.Get("container_123123412")
	lb := r.(*LoadBalancerRegistrationInfo)
	wantedAwsInfo := eureka.AmazonMetadataType{
		PublicHostname: lb.DNSName,
		HostName:       lb.DNSName,
		InstanceID:     lb.DNSName + "_" + strconv.Itoa(int(lb.Port)),
	}
	wantedDCInfo := eureka.DataCenterInfo{
		Name:     eureka.Amazon,
		Metadata: wantedAwsInfo,
	}
	wantedLeaseInfo := eureka.LeaseInfo{
		DurationInSecs: 35,
	}

	wanted := eureka.Instance{
		DataCenterInfo: wantedDCInfo,
		LeaseInfo:      wantedLeaseInfo,
		Port:           int(lb.Port),
		App:            svc.Name,
		IPAddr:         "",
		VipAddress:     "",
		HostName:       lb.DNSName,
		Status:         eureka.UP,
	}

	type args struct {
		service      *bridge.Service
		registration *eureka.Instance
	}
	tests := []struct {
		name string
		args args
		want *eureka.Instance
	}{
		{
			name: "Should match data",
			args: args{service: &svc, registration: &reg},
			want: &wanted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mutateRegistrationInfo(tt.args.service, tt.args.registration)
			val := got.Metadata.GetMap()["has-elbv2"]
			if val != "true" {
				t.Errorf("mutateRegistrationInfo() = %+v, \n Wanted has-elbv2=true in metadata, was %+v", got, val)
			}
			val2 := got.Metadata.GetMap()["elbv2-endpoint"]
			wantVal := lb.DNSName + "_" + strconv.Itoa(int(lb.Port))
			if val2 != wantVal {
				t.Errorf("mutateRegistrationInfo() = %+v, \n Wanted elbv2-endpoint=%v in metadata, was %+v", got, wantVal, val)
			}
			//Overwrite metadata before comparing data structure - we've directly checked the flag we are looking for
			got.Metadata = eureka.InstanceMetadata{}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mutateRegistrationInfo() = %+v, \nwant %+v\n", got, tt.want)
			}
		})
	}
}

// Test_mutateRegistrationInfoExplicitEndpoint - Test that registration struct is returned as expected when you set the host and port
// and that they are used rather than the load balancer lookup
func Test_mutateRegistrationInfoExplicitEndpoint(t *testing.T) {
	initMetadata() // Used from metadata_test.go

	svc := bridge.Service{
		Attrs: map[string]string{
			"eureka_lookup_elbv2_endpoint": "false",
			"eureka_elbv2_hostname":        "hostname-i-set",
			"eureka_elbv2_port":            "65535",
			"eureka_elbv2_targetgroup":     "arn:1234",
			"eureka_datacenterinfo_name":   "AMAZON",
		},
		Name: "app",
		Origin: bridge.ServicePort{
			ContainerID: "123123412",
		},
	}

	awsInfo := eureka.AmazonMetadataType{
		PublicHostname: "i-should-be-changed",
		HostName:       "i-should-be-changed",
		InstanceID:     "i-should-be-changed",
	}

	dcInfo := eureka.DataCenterInfo{
		Name:     eureka.Amazon,
		Metadata: awsInfo,
	}
	wantedLeaseInfo := eureka.LeaseInfo{
		DurationInSecs: 35,
	}

	reg := eureka.Instance{
		DataCenterInfo: dcInfo,
		Port:           5001,
		IPAddr:         "4.3.2.1",
		App:            "app",
		VipAddress:     "4.3.2.1",
		HostName:       "hostname_identifier",
		Status:         eureka.UP,
	}

	// Init LB info cache
	// if things are working correctly, this data won't be used for this test
	thds := []*elbv2.TargetHealthDescription{}
	tgArn := "arn:1234"
	setupCache("123123412", "instance-123", "i-should-not-be-used", 1234, 666, tgArn, thds)

	wantedAwsInfo := eureka.AmazonMetadataType{
		PublicHostname: svc.Attrs["eureka_elbv2_hostname"],
		HostName:       svc.Attrs["eureka_elbv2_hostname"],
		InstanceID:     svc.Attrs["eureka_elbv2_hostname"] + "_" + svc.Attrs["eureka_elbv2_port"],
	}
	wantedDCInfo := eureka.DataCenterInfo{
		Name:     eureka.Amazon,
		Metadata: wantedAwsInfo,
	}

	expectedPort, _ := strconv.Atoi(svc.Attrs["eureka_elbv2_port"])
	wanted := eureka.Instance{
		DataCenterInfo: wantedDCInfo,
		LeaseInfo:      wantedLeaseInfo,
		Port:           expectedPort,
		App:            svc.Name,
		IPAddr:         "",
		VipAddress:     "",
		HostName:       svc.Attrs["eureka_elbv2_hostname"],
		Status:         eureka.UP,
	}

	type args struct {
		service      *bridge.Service
		registration *eureka.Instance
	}
	tests := []struct {
		name string
		args args
		want *eureka.Instance
	}{
		{
			name: "Should match data",
			args: args{service: &svc, registration: &reg},
			want: &wanted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mutateRegistrationInfo(tt.args.service, tt.args.registration)
			val := got.Metadata.GetMap()["has-elbv2"]
			if val != "true" {
				t.Errorf("mutateRegistrationInfo() = %+v, \n Wanted has-elbv2=true in metadata, was %+v", got, val)
			}
			val2 := got.Metadata.GetMap()["elbv2-endpoint"]
			wantVal := svc.Attrs["eureka_elbv2_hostname"] + "_" + svc.Attrs["eureka_elbv2_port"]
			if val2 != wantVal {
				t.Errorf("mutateRegistrationInfo() = %+v, \n Wanted elbv2-endpoint=%v in metadata, was %+v", got, wantVal, val2)
			}
			//Overwrite metadata before comparing data structure - we've directly checked the flag we are looking for
			got.Metadata = eureka.InstanceMetadata{}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mutateRegistrationInfo() = %+v, \nwant %+v\n", got, tt.want)
			}
		})
	}

}

// Test_mutateRegistrationInfoExplicitEndpointCachesData - Test that the elb metadata is cached correctly for use in healthchecks when set explicitly
// and that they are used rather than the load balancer lookup
func Test_mutateRegistrationInfoExplicitEndpointCachesData(t *testing.T) {
	initMetadata() // Used from metadata_test.go

	svc := bridge.Service{
		Attrs: map[string]string{
			"eureka_lookup_elbv2_endpoint": "false",
			"eureka_elbv2_hostname":        "hostname-i-set",
			"eureka_elbv2_port":            "65535",
			"eureka_elbv2_targetgroup":     "arn:1234",
			"eureka_datacenterinfo_name":   "AMAZON",
		},
		Name: "app",
		Origin: bridge.ServicePort{
			ContainerID: "123123412",
		},
	}

	wantedLBDetails := &LoadBalancerRegistrationInfo{
		DNSName: "hostname-i-set",
		IpAddress: "",
		Port: 65535,
		TargetGroupArn: "arn:1234",
		ELBEndpoint: "hostname-i-set_65535",
	}

	awsInfo := eureka.AmazonMetadataType{
		PublicHostname: "i-should-be-changed",
		HostName:       "i-should-be-changed",
		InstanceID:     "i-should-be-changed",
	}

	dcInfo := eureka.DataCenterInfo{
		Name:     eureka.Amazon,
		Metadata: awsInfo,
	}

	reg := eureka.Instance{
		DataCenterInfo: dcInfo,
		Port:           5001,
		IPAddr:         "4.3.2.1",
		App:            "app",
		VipAddress:     "4.3.2.1",
		HostName:       "hostname_identifier",
		Status:         eureka.UP,
	}

	// Init LB info cache
	port := "80"
	healthyTHDs := []*elbv2.TargetHealthDescription{
		{
			HealthCheckPort: &port,
		},
	}
	tgArn := "arn:1234"
	setupTHDCache(tgArn, healthyTHDs)


	t.Run("Should return UP and find LB value in cache", func(t *testing.T) {

		_ = mutateRegistrationInfo(&svc, &reg)
		entry, present := generalCache.Get("container_123123412")
		if !present {
			t.Errorf("Value not in cache")
		}
		if !reflect.DeepEqual(entry, wantedLBDetails) {
			t.Errorf("Should return %v status from cache.  Returned %v", wantedLBDetails, entry)
		}

	})

}



// Test_mutateRegistrationInfoELBv2Only - Test that certain metadata is stripped out when using an ELBv2 only registration setting
func Test_mutateRegistrationInfoELBv2Only(t *testing.T) {
	initMetadata() // Used from metadata_test.go

	svc := bridge.Service{
		Attrs: map[string]string{
			"eureka_elbv2_only_registration": "true",
			"eureka_lookup_elbv2_endpoint":   "false",
			"eureka_datacenterinfo_name":     "AMAZON",
		},
		Name: "app",
		Origin: bridge.ServicePort{
			ContainerID: "123123412",
		},
	}

	awsInfo := eureka.AmazonMetadataType{
		PublicHostname: "i-should-be-changed",
		HostName:       "i-should-be-changed",
		InstanceID:     "i-should-be-changed",
	}

	dcInfo := eureka.DataCenterInfo{
		Name:     eureka.Amazon,
		Metadata: awsInfo,
	}
	wantedLeaseInfo := eureka.LeaseInfo{
		DurationInSecs: 35,
	}

	rawMdInput := []byte(`<is-container>true</is-container>
		<container-id>container-id-goes-here</container-id>
		<container-name>container-name-goes-here</container-name>
		<hudl.version>1.0.0-testingDeployment48</hudl.version>
		<hudl.routes>route/.*|foo/bar/.*|api/special.*</hudl.routes>
		<branch>testingDeployment</branch>
		<aws-instance-id>i-000d95143d83f4ab2</aws-instance-id>
		<elbv2-endpoint>endpoint-goes-here_5051</elbv2-endpoint>`)

	reg := eureka.Instance{
		DataCenterInfo: dcInfo,
		Port:           5001,
		IPAddr:         "4.3.2.1",
		App:            "app",
		VipAddress:     "4.3.2.1",
		HostName:       "hostname_identifier",
		Status:         eureka.UP,
		Metadata: eureka.InstanceMetadata{
			Raw: rawMdInput,
		},
	}
	// Force parsing of metadata
	err, _ := reg.Metadata.GetString("elbv2-endpoint")
	if err != "" {
		t.Errorf("Unable to parse metadata")
	}
	// Init LB info cache
	thds := []*elbv2.TargetHealthDescription{}
	tgArn := "arn:1234"
	setupCache("123123412", "instance-123", "correct-hostname", 1234, 12345, tgArn, thds)

	r, _ := generalCache.Get("container_123123412")
	lb := r.(*LoadBalancerRegistrationInfo)
	wantedAwsInfo := eureka.AmazonMetadataType{
		PublicHostname: lb.DNSName,
		HostName:       lb.DNSName,
		InstanceID:     lb.DNSName + "_" + strconv.Itoa(int(lb.Port)),
	}
	wantedDCInfo := eureka.DataCenterInfo{
		Name:     eureka.Amazon,
		Metadata: wantedAwsInfo,
	}

	wanted := eureka.Instance{
		DataCenterInfo: wantedDCInfo,
		LeaseInfo:      wantedLeaseInfo,
		Port:           int(lb.Port),
		App:            svc.Name,
		IPAddr:         "",
		VipAddress:     "",
		HostName:       lb.DNSName,
		Status:         eureka.UP,
	}

	type args struct {
		service      *bridge.Service
		registration *eureka.Instance
	}
	tests := []struct {
		name string
		args args
		want *eureka.Instance
	}{
		{
			name: "Should match data",
			args: args{service: &svc, registration: &reg},
			want: &wanted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mutateRegistrationInfo(tt.args.service, tt.args.registration)
			CheckMetadata(t, got.Metadata, "has-elbv2", "true")
			wantVal := lb.DNSName + "_" + strconv.Itoa(int(lb.Port))
			CheckMetadata(t, got.Metadata, "elbv2-endpoint", wantVal)
			CheckMetadata(t, got.Metadata, "container-id", "")
			CheckMetadata(t, got.Metadata, "container-name", "")
			CheckMetadata(t, got.Metadata, "aws-instance-id", "")
			//Overwrite metadata before comparing data structure - we've directly checked the flag we are looking for
			got.Metadata = eureka.InstanceMetadata{}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mutateRegistrationInfo() = %+v, \nwant %+v\n", got, tt.want)
			}
		})
	}
}

// Check a metadata string against a wanted value
func CheckMetadata(t *testing.T, md eureka.InstanceMetadata, key string, want string) {
	val := md.GetMap()[key]
	if val != want {
		t.Errorf("Wanted %s=%s in metadata, was %+v", key, want, val)
	}
}
