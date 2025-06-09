package config

import (
	"fmt"
	"github.com/dns4acme/dns4acme/lang/E"
	"math"
	"reflect"
	"testing"
)

func TestIntConverter(t *testing.T) {
	type testCase struct {
		input         any
		expectedValue any
		expectedError E.Error
	}

	ptrVal := int64(1)
	//goland:noinspection GoRedundantConversion
	for name, tc := range map[string]testCase{
		"struct-int": {
			input:         struct{}{},
			expectedValue: int(1),
			expectedError: ErrCannotConvertValue,
		},
		"int-struct": {
			input:         int(1),
			expectedValue: struct{}{},
			expectedError: ErrCannotConvertValue,
		},
		"string-int": {
			input:         "1",
			expectedValue: int(1),
			expectedError: nil,
		},
		"string-int64-small": {
			input:         "-9223372036854775809",
			expectedValue: int64(1),
			expectedError: ErrValueTooSmall,
		},
		"string-int64-large": {
			input:         fmt.Sprintf("%d", uint64(math.MaxInt64)+1),
			expectedValue: int64(1),
			expectedError: ErrValueTooLarge,
		},
		"string-int64-invalid": {
			input:         "asdf",
			expectedValue: int64(1),
			expectedError: ErrCannotConvertValue,
		},
		"int-int": {
			input:         int(1),
			expectedValue: int(1),
			expectedError: nil,
		},
		"int-int-ptr": {
			input:         int(1),
			expectedValue: &ptrVal,
			expectedError: nil,
		},
		"int-ptr-int-ptr": {
			input:         &ptrVal,
			expectedValue: int(1),
			expectedError: nil,
		},
		"uint-int": {
			input:         uint(1),
			expectedValue: int64(1),
			expectedError: nil,
		},
		"uint8-int": {
			input:         uint8(1),
			expectedValue: int64(1),
			expectedError: nil,
		},
		"uint16-int": {
			input:         uint16(1),
			expectedValue: int64(1),
			expectedError: nil,
		},
		"uint32-int": {
			input:         uint32(1),
			expectedValue: int64(1),
			expectedError: nil,
		},
		"uint64-int": {
			input:         uint64(1),
			expectedValue: int64(1),
			expectedError: nil,
		},
		"large-uint64-int": {
			input:         uint64(math.MaxInt64) + 1,
			expectedValue: int64(1),
			expectedError: ErrValueTooLarge,
		},
		"large-int-int8": {
			input:         int(math.MaxInt8) + 1,
			expectedValue: int8(1),
			expectedError: ErrValueTooLarge,
		},
		"small-int-int8": {
			input:         int(math.MinInt8) - 1,
			expectedValue: int8(1),
			expectedError: ErrValueTooSmall,
		},
		"large-int-int16": {
			input:         int(math.MaxInt16) + 1,
			expectedValue: int16(1),
			expectedError: ErrValueTooLarge,
		},
		"small-int-int16": {
			input:         int(math.MinInt16) - 1,
			expectedValue: int16(1),
			expectedError: ErrValueTooSmall,
		},
		"large-int-int32": {
			input:         int(math.MaxInt32) + 1,
			expectedValue: int32(1),
			expectedError: ErrValueTooLarge,
		},
		"small-int-int32": {
			input:         int(math.MinInt32) - 1,
			expectedValue: int32(1),
			expectedError: ErrValueTooSmall,
		},
	} {
		t.Run(name, func(t *testing.T) {
			conv := &intConverter{}
			expectedType := reflect.TypeOf(tc.expectedValue)
			targetValue := reflect.New(expectedType).Elem()

			if err := conv.convert(reflect.ValueOf(tc.input), targetValue); err != nil {
				if tc.expectedError != nil && tc.expectedError != nil == E.Is(err, tc.expectedError) {
					return
				}
				if tc.expectedError == nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				t.Fatalf("Unexpected error: %v, expected: %v", err, tc.expectedError)
			}
			if tc.expectedError != nil {
				t.Fatalf("Expected error, found none: %v", tc.expectedError)
			}
			if targetValue.Kind() == reflect.Pointer {
				targetValue = targetValue.Elem()
			}
			expectedValue := tc.expectedValue
			if v := reflect.ValueOf(expectedValue); v.Kind() == reflect.Pointer {
				expectedValue = v.Elem().Interface()
			}
			if targetValue.Interface() != expectedValue {
				t.Fatalf("Unexpected value: %v, expected: %v", targetValue.Interface(), expectedValue)
			}
		})
	}
}
