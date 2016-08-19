package govk

type errorRequestParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type errorStruct struct {
	Error_code     int                 `json:"error_code"`
	Error_msg      string              `json:"error_msg"`
	Request_params []errorRequestParam `json:"request_params"`
}

type ErrorResponseStruct struct {
	Error errorStruct `json:"error"`
}

// ResponseError holds ErrorResponseStruct of api response.
type ResponseError struct {
	ErrorStruct ErrorResponseStruct
}

func (e ResponseError) Error() string {
	return e.ErrorStruct.Error.Error_msg
}
