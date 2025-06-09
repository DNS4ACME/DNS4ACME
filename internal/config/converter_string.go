package config

import (
	"reflect"
)

type stringConverter struct{}

func (s stringConverter) convert(sourceValue reflect.Value, targetValue reflect.Value) error {
	if sourceValue.Kind() != reflect.String || targetValue.Kind() != reflect.Slice || targetValue.Type().Elem().Kind() != reflect.Uint8 {
		return ErrCannotConvertValue
	}
	targetValue.Set(reflect.ValueOf([]byte(sourceValue.String())))
	return nil
}
