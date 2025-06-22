package E //nolint:revive //We want this to be called E

type unwrappable interface {
	Unwrap() error
}

// HasCode returns true if err or any of its wrapped errors has the specified code.
func HasCode(err error, code string) bool {
	//goland:noinspection GoTypeAssertionOnErrors
	structuredErr, ok := err.(Error) //nolint:errorlint // This is intentional
	if ok && structuredErr.GetCode() == code {
		return true
	}
	u, ok := err.(unwrappable)
	if !ok {
		return false
	}
	return HasCode(u.Unwrap(), code)
}

// Is returns true if the specified err has the same code as the original.
func Is(err error, original Error) bool {
	return HasCode(err, original.GetCode())
}
