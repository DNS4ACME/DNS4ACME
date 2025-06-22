package config

import (
	"errors"
	"log/slog"
	"math"
	"reflect"
	"strconv"
)

type intConverter struct{}

func (i intConverter) convert(sourceValue reflect.Value, targetValue reflect.Value) error {
	if !targetValue.CanSet() {
		return ErrCannotSetTargetValue
	}
	originalTargetValue := targetValue
	targetType := targetValue.Type()
	if targetValue.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
		targetValue = targetValue.Elem()
	}

	var minValue int64
	var maxValue int64
	switch targetType.Kind() {
	case reflect.Int:
		minValue = math.MinInt
		maxValue = math.MaxInt
	case reflect.Int8:
		minValue = math.MinInt8
		maxValue = math.MaxInt8
	case reflect.Int16:
		minValue = math.MinInt16
		maxValue = math.MaxInt16
	case reflect.Int32:
		minValue = math.MinInt32
		maxValue = math.MaxInt32
	case reflect.Int64:
		minValue = math.MinInt64
		maxValue = math.MaxInt64
	default:
		return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String()))
	}
	var val int64
	if sourceValue.Kind() == reflect.Ptr {
		sourceValue = sourceValue.Elem()
	}
	switch sourceValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val = sourceValue.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v := sourceValue.Uint()
		if v > uint64(math.MaxInt64) {
			return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String())).Wrap(ErrValueTooLarge)
		}
		val = int64(v)
	case reflect.String:
		var err error
		str := sourceValue.String()
		val, err = strconv.ParseInt(str, 10, 64)
		if err != nil {
			if errors.Is(err, strconv.ErrRange) {
				if str[0] == '-' {
					return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String())).Wrap(ErrValueTooSmall)
				}
				return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String())).Wrap(ErrValueTooLarge)
			}
			return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String())).Wrap(err)
		}
	default:
		return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String()))
	}

	if val < minValue {
		return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String())).Wrap(ErrValueTooSmall)
	}
	if val > maxValue {
		return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String())).Wrap(ErrValueTooLarge)
	}
	if originalTargetValue.Kind() == reflect.Ptr {
		newValue := reflect.New(targetType)
		newValue.Elem().SetInt(val)
		originalTargetValue.Set(newValue)
	} else {
		targetValue.SetInt(val)
	}
	return nil
}

var _ converter = &intConverter{}
