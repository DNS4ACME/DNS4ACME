package config

import (
	"fmt"
	"log/slog"
	"reflect"
)

type unmarshalTextConverter struct{}

func (u unmarshalTextConverter) convert(sourceValue reflect.Value, targetValue reflect.Value) error {
	if sourceValue.Kind() == reflect.Ptr {
		sourceValue = sourceValue.Elem()
	}
	if sourceValue.Kind() != reflect.String {
		return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String()))
	}
	if targetValue.Kind() == reflect.Ptr && targetValue.IsNil() {
		// We have a nil value, which means we can't unmarshal
		targetValue.Set(reflect.New(targetValue.Type().Elem()))
	}

	unmarshalTextFunc := targetValue.MethodByName("UnmarshalText")
	if !unmarshalTextFunc.IsValid() {
		if targetValue.Kind() != reflect.Ptr {
			// Retry with a reference
			targetValue = targetValue.Addr()
			unmarshalTextFunc = targetValue.MethodByName("UnmarshalText")
			if !unmarshalTextFunc.IsValid() {
				return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String())).Wrap(fmt.Errorf("no UnmarshalText function"))
			}
		} else {
			return ErrCannotConvertValue.WithAttr(slog.String("source", sourceValue.Kind().String())).WithAttr(slog.String("target", targetValue.Kind().String())).Wrap(fmt.Errorf("no UnmarshalText function"))
		}
	}
	if unmarshalTextFunc.Type().NumIn() != 1 {
		return ErrInvalidUnmarshalTextFunction.WithAttr(slog.Int("arguments", unmarshalTextFunc.Type().NumIn()))
	}
	if unmarshalTextFunc.Type().NumOut() != 1 {
		return ErrInvalidUnmarshalTextFunction.WithAttr(slog.Int("return_values", unmarshalTextFunc.Type().NumIn()))
	}

	returnValues := unmarshalTextFunc.Call([]reflect.Value{reflect.ValueOf([]byte(sourceValue.String()))})
	if !returnValues[0].IsNil() {
		err, ok := returnValues[0].Interface().(error)
		if !ok {
			return ErrInvalidUnmarshalTextFunction.WithAttr(slog.String("return_type", fmt.Sprintf("%T", returnValues[0].Interface())))
		}
		return err
	}
	return nil
}

var _ converter = &unmarshalTextConverter{}
