package foundation

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
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

func ExtractHostAndPort(URL string) (string, error) {
	parsedURL, err := url.Parse(URL)
	if err != nil {
		return "", err
	}

	host := parsedURL.Hostname()
	port := parsedURL.Port()

	return fmt.Sprintf("%s:%s", host, port), nil
}
