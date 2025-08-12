package E //nolint:revive //We want this to be called E

import (
	"errors"
	"log/slog"
	"slices"
)

// ToSLogAttr compiles an slog.Attr slice that you can use to pass to slog loggers.
func ToSLogAttr(err error, extra ...any) []any {
	var result []any
	keys := map[string]struct{}{}
	originalErr := err
	for err != nil {
		var structuredErr Error
		if !errors.As(err, &structuredErr) {
			break
		}
		attrs := structuredErr.GetAttrs().Slice()
		slices.Reverse(attrs)
		for _, attr := range attrs {
			if _, ok := keys[attr.Key]; !ok {
				result = append(result, attr)
				keys[attr.Key] = struct{}{}
			}
		}
		err = structuredErr.Unwrap()
	}
	var structuredErr Error
	if !errors.As(originalErr, &structuredErr) {
		return []any{slog.String("error_message", originalErr.Error())}
	}
	res := []any{
		slog.String("error_code", structuredErr.GetCode()),
		slog.String("error_message", structuredErr.GetMessage()),
	}
	cause := structuredErr.Unwrap()
	if cause != nil {
		res = append(res, slog.String("error_cause", cause.Error()))
	}
	return append(extra, append(res, result...)...)
}
