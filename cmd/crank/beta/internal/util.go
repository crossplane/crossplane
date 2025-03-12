package internal

func DereferenceSlice[T any](slice []*T) []T {
	result := make([]T, len(slice))
	for i, element := range slice {
		result[i] = *element
	}
	return result
}
