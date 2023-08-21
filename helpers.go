package foundation

import (
	"fmt"
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
