package problem

import "net/http"

const (
	InternalError   = "Internal Error"
	BadRequest      = "Bad Request"
	Unauthorized    = "Unauthorized"
	Forbidden       = "Forbidden"
	NotFound        = "Not Found"
	Conflict        = "Conflict"
	TooManyRequests = "Too Many Requests"
)

const errorType = "https://latebit.io/az/errors/"

// Details RFC 7807: Problem Details
type Details struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

func NewServerError(err error) Details {
	return Details{
		Type:   errorType,
		Title:  InternalError,
		Status: http.StatusInternalServerError,
		Detail: err.Error(),
	}
}

func NewBadRequest(err error) Details {
	return Details{
		Type:   errorType,
		Title:  BadRequest,
		Status: http.StatusBadRequest,
		Detail: err.Error(),
	}
}

func NewProblem(title string, status int, err error) Details {
	return Details{
		Type:   errorType,
		Status: status,
		Title:  title,
		Detail: err.Error(),
	}
}
