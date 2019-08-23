package bridge

import (
	"net/http"
	"reflect"
	"sort"
	"testing"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type ClientMock struct {
}

func (c *ClientMock) Get(value string) (*http.Response, error) {
	return &http.Response{Body: new(MockedBody)}, nil
}

type MockedBody struct {
	mock.Mock
}

func (m *MockedBody) Close() error {
	return nil
}
func (m *MockedBody) Read(bytes []byte) (int, error) {
	return 0, nil
}

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
	// Setup
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
	// Setup
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

// func TestGetIPFromExternalSource(t *testing.T) {

// 	ipRetryInterval = 0

// 	client = new(ClientMock)

// 	tests := []struct {
// 		name     string
// 		want     string
// 		want1    bool
// 		ipSource string
// 		attempts int
// 	}{
// 		{
// 			name:     "Returns false when server doesn't exist",
// 			ipSource: "http://localhost:1234",
// 			attempts: 1,
// 			want:     "",
// 			want1:    false,
// 		},
// 		{
// 			name:     "Returns correctly when IP found",
// 			ipSource: "http://localhost:1234",
// 			attempts: 1,
// 			want:     "1.2.3.4",
// 			want1:    true,
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			SetExternalIPSource(tt.ipSource)
// 			SetIPLookupRetries(tt.attempts)
// 			got, got1 := GetIPFromExternalSource()
// 			assert.Equal(t, got, tt.want)
// 			assert.Equal(t, got1, tt.want1)
// 		})
// 	}
// }

// func Test_combineTags(t *testing.T) {
// 	type args struct {
// 		tagParts []string
// 	}
// 	tests := []struct {
// 		name string
// 		args args
// 		want []string
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if got := combineTags(tt.args.tagParts...); !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("combineTags() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

func Test_lookupMetaData_ReturnsVarWhenPresent(t *testing.T) {

	// Setup
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

	// Setup
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
	// Setup
	config := dockerapi.Config{Env: []string{
		"SERVICE_FOO=b",
		"SERVICE_BAR=c",
		"NOT_ME=d",
		"SERVICE_FOO_1234=e",
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
	assert.True(t, reflect.DeepEqual(got, withPort))
	assert.True(t, reflect.DeepEqual(got2, portKeys))

}

func Test_serviceMetaData_UseNormalValueWhenNoPort(t *testing.T) {
	// Setup
	config := dockerapi.Config{Env: []string{
		"SERVICE_FOO=b",
		"SERVICE_BAR=c",
		"NOT_ME=d",
		"SERVICE_FOO_1234=e",
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
	assert.True(t, reflect.DeepEqual(got, withoutPort))
	assert.True(t, reflect.DeepEqual(got2, withoutPortKeys))

}

// func Test_servicePort(t *testing.T) {
// 	type args struct {
// 		container *dockerapi.Container
// 		port      dockerapi.Port
// 		published []dockerapi.PortBinding
// 	}
// 	tests := []struct {
// 		name string
// 		args args
// 		want ServicePort
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if got := servicePort(tt.args.container, tt.args.port, tt.args.published); !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("servicePort() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

// func Test_serviceSync(t *testing.T) {
// 	type args struct {
// 		b     *Bridge
// 		quiet bool
// 		newIP string
// 	}
// 	tests := []struct {
// 		name string
// 		args args
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			serviceSync(tt.args.b, tt.args.quiet, tt.args.newIP)
// 		})
// 	}
// }
