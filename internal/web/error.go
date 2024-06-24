package web

func CodeError(code int, err error) error {
	return resError{error: err, code: code}
}

type resError struct {
	error
	code int
}

func (r resError) AppErrCode() int {
	return r.code
}
