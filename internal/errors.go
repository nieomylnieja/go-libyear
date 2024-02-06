package internal

type NotFoundError struct{ err error }

func (e NotFoundError) Error() string { return "not found" }
