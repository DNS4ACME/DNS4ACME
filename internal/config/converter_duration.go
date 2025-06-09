package config

import (
	"reflect"
	"time"
)

type durationConverter struct{}

func (d durationConverter) convert(sourceValue reflect.Value, targetValue reflect.Value) error {
	if sourceValue.Kind() != reflect.String || targetValue.Type().String() != "time.Duration" {
		return ErrCannotConvertValue
	}
	duration, err := time.ParseDuration(sourceValue.String())
	if err != nil {
		return ErrCannotConvertValue.Wrap(err)
	}
	targetValue.SetInt(int64(duration))
	return nil
}
