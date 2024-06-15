package web

import "ve/internal/web/jsonrpc"

func CodeError(code jsonrpc.ErrorCode, err error) jsonrpc.ErrWithAppCode {
	return resError{err, code}
}

type resError struct {
	error
	code jsonrpc.ErrorCode
}

func (r resError) AppErrCode() jsonrpc.ErrorCode {
	return r.code
}
