package config

import (
	"errors"
	"log/slog"
	"reflect"
	"strconv"
)

type floatConverter struct{}

func (f floatConverter) convert(sourceValue reflect.Value, targetValue reflect.Value) error {
	if !targetValue.CanSet() {
		return ErrCannotSetTargetValue
	}
	originalTargetValue := targetValue
	targetType := targetValue.Type()
	if targetValue.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
		targetValue = targetValue.Elem()
	}
	var val float64
	if sourceValue.Kind() == reflect.Ptr {
		sourceValue = sourceValue.Elem()
	}
	switch sourceValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val = float64(sourceValue.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v := sourceValue.Uint()
		val = float64(v)
	case reflect.Float64, reflect.Float32:
		val = sourceValue.Float()
	case reflect.String:
		var err error
		str := sourceValue.String()
		val, err = strconv.ParseFloat(str, 64)
		if err != nil {
			if errors.Is(err, strconv.ErrRange) {
				if str[0] == '-' {
					return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String())).Wrap(ErrValueTooSmall)
				} else {
					return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String())).Wrap(ErrValueTooLarge)
				}
			}
			return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String())).Wrap(err)
		}
	default:
		return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String()))
	}
	if originalTargetValue.Kind() == reflect.Ptr {
		newValue := reflect.New(targetType)
		newValue.Elem().SetFloat(val)
		originalTargetValue.Set(newValue)
	} else {
		targetValue.SetFloat(val)
	}
	return nil
}
