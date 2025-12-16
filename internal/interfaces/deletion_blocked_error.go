package interfaces

type DeletionBlockedError struct {
	Resource   string
	References map[string]int64
}

func (e *DeletionBlockedError) Error() string {
	return "deletion blocked"
}
