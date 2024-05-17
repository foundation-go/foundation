package foundation

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/gomodule/redigo/redis"
)

// AddSuffix adds a suffix to a string, if it doesn't already have it.
func AddSuffix(s string, suffix string) string {
	if s == "" {
		return suffix
	}

	if strings.HasSuffix(s, suffix) {
		return s
	}

	return fmt.Sprintf("%s-%s", s, suffix)
}

// Clone clones an object.
func Clone(obj interface{}) interface{} {
	return reflect.New(reflect.TypeOf(obj)).Interface()
}

func BuildRedisPool(URL string, poolSize int) (*redis.Pool, error) {
	parsedURL, err := url.Parse(URL)
	if err != nil {
		return nil, err
	}

	username := parsedURL.User.Username()
	password, _ := parsedURL.User.Password()

	return &redis.Pool{
		MaxActive: poolSize,
		MaxIdle:   poolSize,
		Wait:      true,
		Dial: func() (redis.Conn, error) {
			return redis.Dial(
				"tcp",
				fmt.Sprintf("%s:%s", parsedURL.Hostname(), parsedURL.Port()),
				redis.DialUsername(username),
				redis.DialPassword(password),
			)
		},
	}, nil
}
