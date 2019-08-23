package bridge

import (
	"reflect"
	"sync"
	"testing"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
)

func TestNewError(t *testing.T) {
	bridge, err := New(nil, "", Config{})
	assert.Nil(t, bridge)
	assert.Error(t, err)
}

func TestNewValid(t *testing.T) {
	Register(new(fakeFactory), "fake")
	// Note: the following is valid for New() since it does not
	// actually connect to docker.
	bridge, err := New(nil, "fake://", Config{})

	assert.NotNil(t, bridge)
	assert.NoError(t, err)
}

func TestBridge_getServicesCopy(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		registry       RegistryAdapter
		docker         *dockerapi.Client
		services       map[string][]*Service
		deadContainers map[string]*DeadContainer
		config         Config
	}
	tests := []struct {
		name   string
		fields fields
		want   map[string][]*Service
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Bridge{
				Mutex:          tt.fields.Mutex,
				registry:       tt.fields.registry,
				docker:         tt.fields.docker,
				services:       tt.fields.services,
				deadContainers: tt.fields.deadContainers,
				config:         tt.fields.config,
			}
			if got := b.getServicesCopy(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bridge.getServicesCopy() = %v, want %v", got, tt.want)
			}
		})
	}
}
