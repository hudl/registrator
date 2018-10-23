package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/hudl/fargo"
	"testing"
	gocache "github.com/patrickmn/go-cache"
)


// Setup the target health descriptions cache specifically
func setupTHDCache(tgArn string, thds []*elbv2.TargetHealthDescription) {
	fn2 := func(thds []*elbv2.TargetHealthDescription) ([]*elbv2.TargetHealthDescription, error) {
		return thds, nil
	}
	GetAndCache(tgArn, thds, fn2, gocache.NoExpiration)
	r, _ := generalCache.Get(tgArn)
	fmt.Printf("THD Cache value now looks like this: %+v\n", r.([]*elbv2.TargetHealthDescription))
}

// Test_testHealth - Test that testHealth mutates the registration details correctly
func Test_testHealth(t *testing.T) {
	initMetadata() // Used from metadata_test.go

	port := "80"
	unhealthyTHDs := []*elbv2.TargetHealthDescription{}
	healthyTHDs := []*elbv2.TargetHealthDescription{
		{
			HealthCheckPort: &port,
		},
	}
	tgArn := "arn:1234"
	containerID := "123123412"
	invalidContainerID := "111111"

	setupCache("123123412", "instance-123", "correct-lb-dnsname", 1234, 9001, tgArn, unhealthyTHDs)

	t.Run("Should return STARTING because of unhealthy targets", func(t *testing.T) {
		RemoveKeyFromCache(tgArn)
		setupTHDCache(tgArn, unhealthyTHDs)
		var previousStatus fargo.StatusType
		eurekaStatus := fargo.UNKNOWN
		wanted := fargo.STARTING
		wantedNow := fargo.STARTING

		change := determineNewEurekaStatus(containerID, eurekaStatus, previousStatus)
		if change.registrationStatus != wanted {
			t.Errorf("Should return %v status for reg status.  Returned %v", wanted, change.registrationStatus)
		}
		if change.newStatus != wantedNow {
			t.Errorf("Should return %v status for previous status.  Returned %v", wantedNow, change.newStatus)
		}
	})

	t.Run("Should return UP because of healthy targets 1", func(t *testing.T) {
		RemoveKeyFromCache(tgArn)
		setupTHDCache(tgArn, healthyTHDs)
		previousStatus := fargo.UNKNOWN
		eurekaStatus := fargo.UNKNOWN
		wanted := fargo.UP
		wantedNow := fargo.UP

		change := determineNewEurekaStatus(containerID, eurekaStatus, previousStatus)
		if change.registrationStatus != wanted {
			t.Errorf("Should return %v status for reg status.  Returned %v", wanted, change.registrationStatus)
		}
		if change.newStatus != wantedNow {
			t.Errorf("Should return %v status for previous status.  Returned %v", wantedNow, change.newStatus)
		}
	})

	t.Run("Should fail gracefully", func(t *testing.T) {
		RemoveKeyFromCache(tgArn)
		setupTHDCache(tgArn, healthyTHDs)
		previousStatus := fargo.UNKNOWN
		eurekaStatus := fargo.UNKNOWN
		wanted := fargo.STARTING
		wantedNow := fargo.UNKNOWN

		change := determineNewEurekaStatus(invalidContainerID, eurekaStatus, previousStatus)
		if change.registrationStatus != wanted {
			t.Errorf("Should return %v status for reg status.  Returned %v", wanted, change.registrationStatus)
		}
		if change.newStatus != wantedNow {
			t.Errorf("Should return %v status for previous status.  Returned %v", wantedNow, change.newStatus)
		}
	})

	t.Run("Should return UP because of eureka status", func(t *testing.T) {
		RemoveKeyFromCache(tgArn)
		setupTHDCache(tgArn, unhealthyTHDs)

		previousStatus := fargo.UNKNOWN
		eurekaStatus := fargo.UP
		wantedReg := fargo.UP
		wantedNow := fargo.UP

		change := determineNewEurekaStatus(containerID, eurekaStatus, previousStatus)
		if change.registrationStatus != wantedReg {
			t.Errorf("Should return %v status for reg status.  Returned %v", wantedReg, change.registrationStatus)
		}
		if change.newStatus != wantedNow {
			t.Errorf("Should return %v status for previous status.  Returned %v", wantedNow, change.newStatus)
		}
	})

	t.Run("Should return UP because of healthy targets 2", func(t *testing.T) {
		RemoveKeyFromCache(tgArn)
		setupTHDCache(tgArn, healthyTHDs)

		previousStatus := fargo.STARTING
		eurekaStatus := fargo.STARTING
		wantedReg := fargo.UP
		wantedNow := fargo.UP

		change := determineNewEurekaStatus(containerID, eurekaStatus, previousStatus)
		if change.registrationStatus != wantedReg {
			t.Errorf("Should return %v status for reg status.  Returned %v", wantedReg, change.registrationStatus)
		}
		if change.newStatus != wantedNow {
			t.Errorf("Should return %v status for previous status.  Returned %v", wantedNow, change.newStatus)
		}
	})

}

