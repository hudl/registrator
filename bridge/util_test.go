package bridge

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"sort"
	"testing"
	"time"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestEscapedComma(t *testing.T) {
	cases := []struct {
		Tag      string
		Expected []string
	}{
		{
			Tag:      "",
			Expected: []string{},
		},
		{
			Tag:      "foobar",
			Expected: []string{"foobar"},
		},
		{
			Tag:      "foo,bar",
			Expected: []string{"foo", "bar"},
		},
		{
			Tag:      "foo\\,bar",
			Expected: []string{"foo,bar"},
		},
		{
			Tag:      "foo,bar\\,baz",
			Expected: []string{"foo", "bar,baz"},
		},
		{
			Tag:      "\\,foobar\\,",
			Expected: []string{",foobar,"},
		},
		{
			Tag:      ",,,,foo,,,bar,,,",
			Expected: []string{"foo", "bar"},
		},
		{
			Tag:      ",,,,",
			Expected: []string{},
		},
		{
			Tag:      ",,\\,,",
			Expected: []string{","},
		},
	}

	for _, c := range cases {
		results := recParseEscapedComma(c.Tag)
		sort.Strings(c.Expected)
		sort.Strings(results)
		assert.EqualValues(t, c.Expected, results)
	}
}

func Test_mapDefault_ReturnsDefaultWhenMissing(t *testing.T) {
	// Arrange
	var got string
	key := "bla"
	defaultStr := "my-default"
	metadata := make(map[string]string)
	metadata["test-item"] = "test-value"

	// Act
	t.Run("Returns default when key is missing", func(t *testing.T) {
		got = mapDefault(metadata, key, defaultStr)
	})
	// Assert
	assert.Equal(t, got, defaultStr)
}

func Test_mapDefault_ReturnsValueWhenPresent(t *testing.T) {
	// Arrange
	var got string
	key := "test-item"
	expectedValue := "test-value"
	defaultStr := "my-default"
	metadata := make(map[string]string)
	metadata["test-item"] = "test-value"

	// Act
	t.Run("Returns default when key is missing", func(t *testing.T) {
		got = mapDefault(metadata, key, defaultStr)
	})
	// Assert
	assert.Equal(t, got, expectedValue)
}

type ClientMock struct {
	mock.Mock
}

func (c *ClientMock) Get(value string) (*http.Response, error) {
	args := c.Called(value)
	body := ioutil.NopCloser(bytes.NewReader(args.Get(0).([]byte)))
	return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
}

func TestGetIPFromExternalSource_ReturnsIPCorrectly(t *testing.T) {
	// Arrange
	ipRetryInterval = 0
	mockobj := ClientMock{}
	client = &mockobj
	SetExternalIPSource("http://localhost:1234")
	SetIPLookupRetries(1)

	var got string
	var got1 bool
	mockobj.On("Get", "http://localhost:1234").Return([]byte("1.2.3.4"))

	// Act
	t.Run("Returns IP correctly", func(t *testing.T) {
		got, got1 = GetIPFromExternalSource()
	})

	// Assert
	mockobj.AssertExpectations(t)
	assert.Equal(t, "1.2.3.4", got)
	assert.Equal(t, true, got1)
}

func TestGetIPFromExternalSource_ReturnsFalseWhenServerDoesNotExist(t *testing.T) {
	// Arrange
	ipRetryInterval = 0
	SetExternalIPSource("http://localhost:1234")
	SetIPLookupRetries(1)
	client = &http.Client{
		Timeout: time.Duration(1 * time.Second),
	}

	var got string
	var got1 bool

	// Act
	t.Run("Returns false when server does not exist", func(t *testing.T) {
		got, got1 = GetIPFromExternalSource()
	})

	// Assert
	assert.Equal(t, "", got)
	assert.Equal(t, false, got1)
}

func Test_lookupMetaData_ReturnsVarWhenPresent(t *testing.T) {
	// Arrange
	config := dockerapi.Config{Env: []string{"MY_VAR=a", "MY_VAR2=b"}}
	var got string
	key := "MY_VAR"

	// Act
	t.Run("Returns value when present", func(t *testing.T) {
		got = lookupMetaData(&config, key)
	})

	// Assert
	assert.Equal(t, "a", got)
}

func Test_lookupMetaData_ReturnsEmptyWhenNotPresent(t *testing.T) {
	// Arrange
	config := dockerapi.Config{Env: []string{"MY_VAR=a", "MY_VAR2=b"}}
	var got string
	key := "NOT_HERE"

	// Act
	t.Run("Returns value when present", func(t *testing.T) {
		got = lookupMetaData(&config, key)
	})

	// Assert
	assert.Equal(t, "", got)
}

func Test_serviceMetaData_PortValueTakesPrecedence(t *testing.T) {
	// Arrange
	config := dockerapi.Config{Env: []string{
		"SERVICE_FOO=b",
		"SERVICE_BAR=c",
		"NOT_ME=d",
		"SERVICE_1234_FOO=e",
	},
		Labels: map[string]string{
			"SERVICE_FOO": "a",
		},
	}
	var withPort = map[string]string{
		"foo": "e",
		"bar": "c",
	}
	var portKeys = map[string]bool{
		"foo": true,
	}
	var got map[string]string
	var got2 map[string]bool

	// Act
	t.Run("Port value is retrieved in precedence", func(t *testing.T) {
		got, got2 = serviceMetaData(&config, "1234")

	})
	// Assert
	assert.EqualValues(t, withPort, got)
	assert.EqualValues(t, portKeys, got2)

}

func Test_serviceMetaData_UseNormalValueWhenNoPort(t *testing.T) {
	// Arrange
	config := dockerapi.Config{Env: []string{
		"SERVICE_FOO=b",
		"SERVICE_BAR=c",
		"NOT_ME=d",
		"SERVICE_1234_FOO=e",
	},
		Labels: map[string]string{
			"SERVICE_FOO": "a",
		},
	}
	var withoutPort = map[string]string{
		"foo": "b",
		"bar": "c",
	}
	var withoutPortKeys = map[string]bool{}
	var got map[string]string
	var got2 map[string]bool

	// Act
	t.Run("Normal value used when port not specified", func(t *testing.T) {
		got, got2 = serviceMetaData(&config, "")

	})

	// Assert
	assert.EqualValues(t, withoutPort, got)
	assert.EqualValues(t, withoutPortKeys, got2)
}

func Test_serviceMetaData_EurekaMetadataSetCorrectly(t *testing.T) {
	// Arrange
	config := dockerapi.Config{Env: []string{
		"NOT_ME=d",
		"SERVICE_FOO=b",
		"SERVICE_1234_FOO=e",
		"SERVICE_EUREKA_METADATA_branch=testbranch",
	},
		Labels: map[string]string{
			"SERVICE_FOO": "a",
		},
	}
	var withoutPort = map[string]string{
		"eureka_metadata_branch": "testbranch",
		"foo":                    "b",
	}
	var withoutPortKeys = map[string]bool{}
	var got map[string]string
	var got2 map[string]bool

	// Act
	t.Run("Eureka metadata is correctly retrieved", func(t *testing.T) {
		got, got2 = serviceMetaData(&config, "")

	})

	// Assert
	assert.EqualValues(t, withoutPort, got)
	assert.EqualValues(t, withoutPortKeys, got2)
}
