package aws

import (
	"github.com/patrickmn/go-cache"
	"reflect"
	"time"
)

var generalCache = cache.New(DEFAULT_EXP_TIME, DEFAULT_EXP_TIME)

type Any interface{}

//
// Provide a general caching mechanism for any function that lasts a few seconds.
//
func GetAndCache(key string, input Any, f Any, cacheTime time.Duration) (Any, error) {

	vf := reflect.ValueOf(f)
	vinput := reflect.ValueOf(input)

	result, found := generalCache.Get(key)
	if !found {
		caller := vf.Call([]reflect.Value{vinput})
		output := caller[0].Interface()
		err, _ := caller[1].Interface().(error)
		if err == nil {
			generalCache.Set(key, output, cacheTime)
			return output, nil
		}
		return nil, err
	}
	return result, nil
}

func AddToCache(key string, value Any, cacheTime time.Duration) {
	generalCache.Set(key, value, cacheTime)
}

// RemoveKeyFromCache : Delete any entry cache in the cache for this key
func RemoveKeyFromCache(key string) {
	generalCache.Delete(key)
}
