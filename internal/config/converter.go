package config

import "reflect"

type converter interface {
	convert(source reflect.Value, targetValue reflect.Value) error
}

var converters = []converter{
	&unmarshalTextConverter{},
	&stringSliceConverter{},
	&intConverter{},
	&uintConverter{},
	&floatConverter{},
	&stringConverter{},
	&durationConverter{},
}
