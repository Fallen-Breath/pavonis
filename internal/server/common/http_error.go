package common

type HttpError struct {
	Status  int
	Message string
}

var _ error = &HttpError{}

func (e *HttpError) Error() string {
	return e.Message
}

func NewHttpError(status int, message string) *HttpError {
	return &HttpError{
		Status:  status,
		Message: message,
	}
}
