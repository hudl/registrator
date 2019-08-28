package bridge

import (
	"net/url"

	"github.com/stretchr/testify/mock"
)

type fakeFactory struct{}

func (f *fakeFactory) New(uri *url.URL) RegistryAdapter {
	return &fakeAdapter{}
}

type fakeAdapter struct {
	mock.Mock
}

func (f *fakeAdapter) Ping() error {
	args := f.Called()
	return args.Error(0)
}
func (f *fakeAdapter) Register(service *Service) error {
	args := f.Called(service)
	return args.Error(0)
}
func (f *fakeAdapter) Deregister(service *Service) error {
	args := f.Called(service)
	return args.Error(0)
}
func (f *fakeAdapter) Refresh(service *Service) error {
	args := f.Called(service)
	return args.Error(0)
}
func (f *fakeAdapter) Services() ([]*Service, error) {
	args := f.Called()
	return args.Get(0).([]*Service), nil
}
