package service

func OverrideValidateApplicationIdentifierForTest(
	fn func(string, string) (error, bool),
) (restore func()) {

	prev := validateApplicationIdentifierFn
	validateApplicationIdentifierFn = fn

	return func() {
		validateApplicationIdentifierFn = prev
	}
}
