package aws

type lookupValues struct {
	InstanceID  string
	Port        int64
	ClusterName string
	ServiceName string
	TaskArn     string
}

// LoadBalancerRegistrationInfo represents registration details for a ELBv2 endpoint
type LoadBalancerRegistrationInfo struct {
	DNSName        string
	Port           int
	ELBEndpoint    string
	TargetGroupArn string
	IpAddress      string
	VipAddress     string
}

// HasNoLoadBalancer - Special error type for when container has no load balancer
type HasNoLoadBalancer struct {
	message string
}
