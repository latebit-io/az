package subjects

import "fmt"

type SubjectDuplicateError struct {
	Value string `json:"value"`
}

func (e SubjectDuplicateError) Error() string {
	return fmt.Sprintf("duplicate subject: '%s' already exists", e.Value)
}

type SubjectNotFoundError struct {
	Value string `json:"value"`
}

func (e SubjectNotFoundError) Error() string {
	return fmt.Sprintf("subject not found: %s", e.Value)
}

type InvalidSubjectError struct {
	Value string `json:"value"`
}

func (e InvalidSubjectError) Error() string {
	return fmt.Sprintf("invalid subject: %s", e.Value)
}
