package check

import "fmt"

type InvalidCheckError struct {
	Value string `json:"value"`
}

func (e InvalidCheckError) Error() string {
	return fmt.Sprintf("invalid check: %s", e.Value)
}
