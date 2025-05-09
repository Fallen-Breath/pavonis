package utils

func ToPtr[T any](obj T) *T {
	return &obj
}
