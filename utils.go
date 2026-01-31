package main

func Ifel[T any](ok bool, a T, b T) T {
	if ok {
		return a
	}
	return b
}

func BoolPtr(b bool) *bool {
	return &b
}
