package aws

type lookupValues struct {
	InstanceID  string
	Port        int64
	ClusterName string
	ServiceName string
	TaskArn     string
}

// LBInfo represents a ELBv2 endpoint
type LBInfo struct {
	DNSName        string
	Port           int64
	TargetGroupArn string
}

// HasNoLoadBalancer - Special error type for when container has no load balancer
type HasNoLoadBalancer struct {
	message string
}
