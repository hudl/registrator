package bridge

import (
	"testing"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var adapterUri = "fake://"

var config = Config{
	HostIp:       "1.2.3.4",
	Internal:     false,
	Cleanup:      true,
	RequireLabel: false,
}

type MockDockerClient struct {
	mock.Mock
}

func (m *MockDockerClient) ListContainers(opts dockerapi.ListContainersOptions) ([]dockerapi.APIContainers, error) {
	args := m.Called(opts)
	return nil, args.Error(1)
}
func (m *MockDockerClient) InspectContainer(c string) (*dockerapi.Container, error) {
	args := m.Called(c)
	return nil, args.Error(1)
}

func Test_Initialize(t *testing.T) {

	var docker = MockDockerClient{}
	Register(new(fakeFactory), "fake")
	newBridge, err := New(&docker, adapterUri, config)

	t.Run("Test Initialize", func(t *testing.T) {
		Initialize(newBridge)
	})
	assert.NotNil(t, newBridge)
	assert.NoError(t, err)

}

// func Test_channelRun(t *testing.T) {
// 	type args struct {
// 		bridge *Bridge
// 	}
// 	tests := []struct {
// 		name string
// 		args args
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			channelRun(tt.args.bridge)
// 		})
// 	}
// }

func Test_reregisterService_SetsCorrectValues(t *testing.T) {
	// Setup
	service := Service{IP: "1.2.3.4"}
	wantedService := Service{IP: "5.6.7.8", Origin: ServicePort{HostIP: "5.6.7.8"}}
	adapter := fakeAdapter{}
	newIP := "5.6.7.8"

	adapter.On("Deregister", &service).Return(nil)
	adapter.On("Register", &service).Return(nil)

	// Act
	t.Run("New IP is correctly updated", func(t *testing.T) {
		reregisterService(&adapter, &service, newIP)
	})

	// Assert
	assert.EqualValues(t, service, wantedService)
}

func Test_reregisterService_ReRegistersWithAdapter(t *testing.T) {

	// Setup
	service := Service{IP: "1.2.3.4"}
	adapter := fakeAdapter{}

	newIP := "5.6.7.8"

	adapter.On("Deregister", &service).Return(nil)
	adapter.On("Register", &service).Return(nil)

	// Act
	t.Run("Test registers with adapter", func(t *testing.T) {
		reregisterService(&adapter, &service, newIP)
	})

	// Assert
	adapter.AssertExpectations(t)
	adapter.AssertCalled(t, "Deregister", &service)
	adapter.AssertCalled(t, "Register", &service)

}

func Test_reregisterService_DoesNotDeregisterWithNoIp(t *testing.T) {

	// Setup
	service := Service{IP: "1.2.3.4"}
	adapter := fakeAdapter{}
	newIP := ""

	adapter.On("Register", &service).Return(nil)

	// Act
	t.Run("Test registers with adapter", func(t *testing.T) {
		reregisterService(&adapter, &service, newIP)
	})

	// Assert
	adapter.AssertExpectations(t)
	adapter.AssertNotCalled(t, "Deregister", &service)
	adapter.AssertCalled(t, "Register", &service)

}

func Test_cleanupServices_DoesntRemoveNonMatchingService(t *testing.T) {
	// Setup
	var docker = MockDockerClient{}
	Register(new(fakeFactory), "fake")
	newBridge, err := New(&docker, adapterUri, config)
	keepMe := Service{ID: "keep-me-please-please"}
	var danglingServices = []*Service{
		&keepMe,
	}
	var expectedServices = map[string][]*Service{"test1": {
		&keepMe,
	}}
	newBridge.services["test1"] = []*Service{&keepMe}

	// Act
	t.Run("Cleanup", func(t *testing.T) {
		cleanupServices(newBridge, danglingServices)
	})

	// Assert
	assert.EqualValues(t, expectedServices, newBridge.services)
	assert.NotNil(t, newBridge)
	assert.NoError(t, err)
}

// func Test_serviceSync(t *testing.T) {
// 	type args struct {
// 		message SyncMessage
// 		b       *Bridge
// 	}
// 	tests := []struct {
// 		name string
// 		args args
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			serviceSync(tt.args.message, tt.args.b)
// 		})
// 	}
// }
