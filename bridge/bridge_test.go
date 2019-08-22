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

func TestNew(t *testing.T) {
	type args struct {
		docker     *dockerapi.Client
		adapterUri string
		config     Config
	}
	tests := []struct {
		name    string
		args    args
		want    *Bridge
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.args.docker, tt.args.adapterUri, tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBridge_Ping(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		registry       RegistryAdapter
		docker         *dockerapi.Client
		services       map[string][]*Service
		deadContainers map[string]*DeadContainer
		config         Config
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
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
			if err := b.Ping(); (err != nil) != tt.wantErr {
				t.Errorf("Bridge.Ping() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBridge_Add(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		registry       RegistryAdapter
		docker         *dockerapi.Client
		services       map[string][]*Service
		deadContainers map[string]*DeadContainer
		config         Config
	}
	type args struct {
		containerId string
		ipToUse     string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
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
			b.Add(tt.args.containerId, tt.args.ipToUse)
		})
	}
}

func TestBridge_Remove(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		registry       RegistryAdapter
		docker         *dockerapi.Client
		services       map[string][]*Service
		deadContainers map[string]*DeadContainer
		config         Config
	}
	type args struct {
		containerId string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
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
			b.Remove(tt.args.containerId)
		})
	}
}

func TestBridge_RemoveOnExit(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		registry       RegistryAdapter
		docker         *dockerapi.Client
		services       map[string][]*Service
		deadContainers map[string]*DeadContainer
		config         Config
	}
	type args struct {
		containerId string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
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
			b.RemoveOnExit(tt.args.containerId)
		})
	}
}

func TestBridge_Refresh(t *testing.T) {
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
			b.Refresh()
		})
	}
}

func TestBridge_PruneDeadContainers(t *testing.T) {
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
			b.PruneDeadContainers()
		})
	}
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

func TestBridge_Sync(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		registry       RegistryAdapter
		docker         *dockerapi.Client
		services       map[string][]*Service
		deadContainers map[string]*DeadContainer
		config         Config
	}
	type args struct {
		quiet bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
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
			b.Sync(tt.args.quiet)
		})
	}
}

func TestBridge_AllocateNewIPToServices(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		registry       RegistryAdapter
		docker         *dockerapi.Client
		services       map[string][]*Service
		deadContainers map[string]*DeadContainer
		config         Config
	}
	type args struct {
		ip string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
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
			b.AllocateNewIPToServices(tt.args.ip)
		})
	}
}

func TestBridge_deleteDeadContainer(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		registry       RegistryAdapter
		docker         *dockerapi.Client
		services       map[string][]*Service
		deadContainers map[string]*DeadContainer
		config         Config
	}
	type args struct {
		containerId string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
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
			b.deleteDeadContainer(tt.args.containerId)
		})
	}
}

func TestBridge_appendService(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		registry       RegistryAdapter
		docker         *dockerapi.Client
		services       map[string][]*Service
		deadContainers map[string]*DeadContainer
		config         Config
	}
	type args struct {
		containerId string
		service     *Service
	}
	tests := []struct {
		name   string
		fields fields
		args   args
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
			b.appendService(tt.args.containerId, tt.args.service)
		})
	}
}

func TestBridge_add(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		registry       RegistryAdapter
		docker         *dockerapi.Client
		services       map[string][]*Service
		deadContainers map[string]*DeadContainer
		config         Config
	}
	type args struct {
		containerId string
		quiet       bool
		newIP       string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
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
			b.add(tt.args.containerId, tt.args.quiet, tt.args.newIP)
		})
	}
}

func TestBridge_newService(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		registry       RegistryAdapter
		docker         *dockerapi.Client
		services       map[string][]*Service
		deadContainers map[string]*DeadContainer
		config         Config
	}
	type args struct {
		port    ServicePort
		isgroup bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Service
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
			if got := b.newService(tt.args.port, tt.args.isgroup); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bridge.newService() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBridge_remove(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		registry       RegistryAdapter
		docker         *dockerapi.Client
		services       map[string][]*Service
		deadContainers map[string]*DeadContainer
		config         Config
	}
	type args struct {
		containerId string
		deregister  bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
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
			b.remove(tt.args.containerId, tt.args.deregister)
		})
	}
}

func TestBridge_shouldRemove(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		registry       RegistryAdapter
		docker         *dockerapi.Client
		services       map[string][]*Service
		deadContainers map[string]*DeadContainer
		config         Config
	}
	type args struct {
		containerId string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
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
			if got := b.shouldRemove(tt.args.containerId); got != tt.want {
				t.Errorf("Bridge.shouldRemove() = %v, want %v", got, tt.want)
			}
		})
	}
}
