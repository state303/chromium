package chromium

// Predicate is a generic function that examines an item of type T matches or not, mainly to delegate logic for switch.
type Predicate[T any] func(item T) bool
