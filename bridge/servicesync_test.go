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
	return args.Get(0).([]dockerapi.APIContainers), nil
}
func (m *MockDockerClient) InspectContainer(c string) (*dockerapi.Container, error) {
	args := m.Called(c)
	return args.Get(0).(*dockerapi.Container), nil
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

func Test_reregisterService_SetsCorrectValues(t *testing.T) {
	// Arrange
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

	// Arrange
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

	// Arrange
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
	Hostname = "test"
	keepMe := Service{ID: "keep-me-please-please", Name: "test1"}
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

func Test_cleanupServices_RemovesMatchingService(t *testing.T) {
	// Setup
	var docker = MockDockerClient{}
	var adapter = &fakeAdapter{}
	Register(new(fakeFactory), "fake")
	newBridge, err := New(&docker, adapterUri, config)
	newBridge.registry = adapter
	Hostname = "test"
	keepMe := Service{ID: "keep-me-please-please", Name: "test1"}
	fakeContainer := dockerapi.Container{ID: "bla", Name: "test"}
	deleteMe := Service{ID: "test:test:0", Name: "test2", Origin: ServicePort{container: &fakeContainer}}

	var danglingServices = []*Service{
		&keepMe,
		&deleteMe,
	}
	newBridge.services["test1"] = []*Service{&keepMe}
	newBridge.services["test2"] = []*Service{&deleteMe}
	adapter.On("Deregister", &deleteMe).Return(nil)

	// Act
	t.Run("Cleanup", func(t *testing.T) {
		cleanupServices(newBridge, danglingServices)
	})

	// Assert
	assert.NotNil(t, newBridge)
	assert.NoError(t, err)
	adapter.AssertCalled(t, "Deregister", &deleteMe)

}

func Test_serviceSync_ReregisterIsCalled(t *testing.T) {
	// Arrange
	var docker = MockDockerClient{}
	var adapter = &fakeAdapter{}
	Register(new(fakeFactory), "fake")
	newBridge, err := New(&docker, adapterUri, config)
	newBridge.registry = adapter
	Hostname = "test"

	nonExitedContainers := []dockerapi.APIContainers{
		{ID: "i-didnt-exit"},
	}
	service2 := Service{ID: "i-didnt-exit", Name: "test2"}

	var expectedServices = map[string][]*Service{"i-didnt-exit": {
		&service2,
	}}
	message := SyncMessage{Quiet: false, IP: "5.6.7.8"}

	docker.On("ListContainers", mock.AnythingOfType("ListContainersOptions")).Return(nonExitedContainers)
	adapter.On("Services").Return([]*Service{}, nil)
	adapter.On("Deregister", &service2).Return(nil)
	adapter.On("Register", &service2).Return(nil)

	newBridge.services["i-didnt-exit"] = []*Service{&service2}

	// Act
	t.Run("Testing service sync", func(t *testing.T) {
		serviceSync(message, newBridge)
	})

	// Assert
	assert.NotNil(t, newBridge)
	assert.NoError(t, err)
	adapter.AssertCalled(t, "Deregister", &service2)
	adapter.AssertCalled(t, "Register", &service2)
	assert.EqualValues(t, expectedServices, newBridge.services)
	adapter.AssertExpectations(t)
	docker.AssertExpectations(t)

}

func Test_serviceSync_ContainersAreCleanedUp(t *testing.T) {
	// Arrange
	var docker = MockDockerClient{}
	var adapter = &fakeAdapter{}
	Register(new(fakeFactory), "fake")
	newBridge, err := New(&docker, adapterUri, config)
	newBridge.registry = adapter
	Hostname = "test"

	containers := []dockerapi.APIContainers{
		{ID: "im-gone"},
		{ID: "i-didnt-exit"},
	}
	container1 := dockerapi.Container{ID: "im-gone", State: dockerapi.State{Running: false}}

	nonExitedContainers := []dockerapi.APIContainers{
		{ID: "i-didnt-exit"},
	}
	service1 := Service{ID: "im-gone", Name: "test1"}
	service2 := Service{ID: "i-didnt-exit", Name: "test2"}

	opts1 := dockerapi.ListContainersOptions{}
	opts2 := dockerapi.ListContainersOptions{Filters: filters}

	message := SyncMessage{Quiet: false, IP: "5.6.7.8"}

	docker.On("ListContainers", opts1).Return(containers)
	docker.On("ListContainers", opts2).Return(nonExitedContainers)
	docker.On("InspectContainer", "im-gone").Return(&container1)
	adapter.On("Services").Return([]*Service{}, nil)
	adapter.On("Deregister", &service2).Return(nil)
	adapter.On("Register", &service2).Return(nil)
	adapter.On("Deregister", &service1).Return(nil)
	adapter.On("Register", &service1).Return(nil).Once()

	newBridge.services["im-gone"] = []*Service{&service1}
	newBridge.services["i-didnt-exit"] = []*Service{&service2}

	// Act
	t.Run("Testing service sync", func(t *testing.T) {
		serviceSync(message, newBridge)
	})

	// Assert
	assert.NotNil(t, newBridge)
	assert.NoError(t, err)
	adapter.AssertCalled(t, "Deregister", &service1)
	adapter.AssertCalled(t, "Register", &service2)
	adapter.AssertExpectations(t)
	docker.AssertExpectations(t)

}
