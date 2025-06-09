package config

import (
	"github.com/dns4acme/dns4acme/lang/E"
)

var ErrNoCLIOptions = E.New("NO_CLI_OPTIONS", "no cli options provided")
var ErrUnexpectedCLIOption = E.New("UNEXPECTED_CLI_OPTION", "unexpected cli option")
var ErrDuplicateOption = E.New("DUPLICATE_OPTION", "duplicate option")

var ErrCannotConvertValue = E.New("CANNOT_CONVERT_VALUE", "cannot convert value")
var ErrInvalidUnmarshalTextFunction = E.New("INVALID_UNMARSHAL_TEXT_FUNCTION", "invalid UnmarshalText function (incorrect return types)")
var ErrIncompatibleValue = E.New("INCOMPATIBLE_VALUE", "incompatible value")
var ErrValueTooSmall = E.New("VALUE_TOO_SMALL", "value too small")
var ErrValueTooLarge = E.New("VALUE_TOO_LARGE", "value too large")
var ErrCannotSetTargetValue = E.New("CANNOT_SET_TARGET_VALUE", "cannot set target value")
var ErrNoSuchOption = E.New("NO_SUCH_OPTION", "no such option")
