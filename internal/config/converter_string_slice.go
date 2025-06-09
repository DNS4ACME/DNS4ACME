package config

import (
	"reflect"
	"strings"
)

type stringSliceConverter struct{}

func (s stringSliceConverter) convert(source reflect.Value, targetValue reflect.Value) error {
	if source.Kind() != reflect.String || targetValue.Kind() != reflect.Slice || targetValue.Type().Elem().Kind() != reflect.String {
		return ErrCannotConvertValue
	}
	targetValue.Set(reflect.ValueOf(strings.Split(source.String(), ",")))
	return nil
}
