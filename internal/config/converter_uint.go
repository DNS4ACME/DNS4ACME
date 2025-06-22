package config

import (
	"errors"
	"log/slog"
	"math"
	"reflect"
	"strconv"
)

type uintConverter struct{}

func (u uintConverter) convert(sourceValue reflect.Value, targetValue reflect.Value) error {
	if targetValue.Kind() == reflect.Ptr {
		targetValue = targetValue.Elem()
	}
	if !targetValue.CanSet() {
		return ErrCannotSetTargetValue
	}

	var maxValue uint64
	switch targetValue.Kind() {
	case reflect.Uint:
		maxValue = math.MaxInt
	case reflect.Uint8:
		maxValue = math.MaxInt8
	case reflect.Uint16:
		maxValue = math.MaxUint16
	case reflect.Uint32:
		maxValue = math.MaxUint32
	case reflect.Uint64:
		maxValue = math.MaxUint64
	default:
		return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String()))
	}
	var val uint64
	switch sourceValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if sourceValue.Int() < 0 {
			return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String())).Wrap(ErrValueTooSmall)
		}
		val = sourceValue.Uint()
	case reflect.String:
		var err error
		str := sourceValue.String()
		val, err = strconv.ParseUint(str, 10, 64)
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

	if val > maxValue {
		return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String())).Wrap(ErrValueTooLarge)
	}
	targetValue.SetUint(val)
	return nil
}

var _ converter = &intConverter{}
