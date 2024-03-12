package common

func Remove[T any](slice []T, index int) []T {
	return append(slice[:index], slice[index+1:]...)
}

func Filter[T any](slice []T, f func(T) bool) []T {
	result := []T{}
	for _, item := range slice {
		if f(item) {
			result = append(result, item)
		}
	}
	return result
}
