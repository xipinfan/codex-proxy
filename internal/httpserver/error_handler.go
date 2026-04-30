package httpserver

import (
	"encoding/json"
	"strings"

	"github.com/valyala/fasthttp"
)

type errorResponse struct {
	Error errorObject `json:"error"`
}

type errorObject struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

func parseErrorHandler(ctx *fasthttp.RequestCtx, err error) {
	status := fasthttp.StatusBadRequest
	msg := "Invalid HTTP request. If this request contains images, verify the request body size and encoding."
	code := "invalid_http_request"

	if isBodyTooLargeError(err) {
		status = fasthttp.StatusRequestEntityTooLarge
		msg = "Request body too large. Increase listen-max-request-body-bytes or reduce/compress images."
		code = "request_body_too_large"
	}

	writeJSONError(ctx, status, msg, code)
}

func isBodyTooLargeError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "body size exceeds") ||
		strings.Contains(s, "too large") ||
		strings.Contains(s, "cannot read request body")
}

func writeJSONError(ctx *fasthttp.RequestCtx, status int, message, code string) {
	body, marshalErr := json.Marshal(errorResponse{
		Error: errorObject{
			Message: message,
			Type:    "invalid_request_error",
			Code:    code,
		},
	})
	if marshalErr != nil {
		body = []byte(`{"error":{"message":"Invalid HTTP request.","type":"invalid_request_error","code":"invalid_http_request"}}`)
		status = fasthttp.StatusBadRequest
	}

	ctx.Response.Reset()
	ctx.SetStatusCode(status)
	ctx.SetContentType("application/json")
	ctx.SetBody(body)
}
