package web

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

const maxRequestBytes = 1 << 20

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func writeError(response http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := domain.ErrorCodeOf(err)
	switch code {
	case domain.CodeValidation:
		status = http.StatusBadRequest
	case domain.CodeForbidden:
		status = http.StatusForbidden
	case domain.CodeNotFound:
		status = http.StatusNotFound
	case domain.CodeConflict, domain.CodeState, domain.CodeImmutable, domain.CodeDuplicate, domain.CodeIntegrity:
		status = http.StatusConflict
	}
	message := "服务内部错误"
	if code != "INTERNAL_ERROR" {
		message = err.Error()
	}
	result := errorResponse{}
	result.Error.Code, result.Error.Message = string(code), message
	writeJSON(response, status, result)
}

func decodeJSON(response http.ResponseWriter, request *http.Request, target any) error {
	contentType := request.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		return domain.NewError(domain.CodeValidation, "Content-Type 必须为 application/json")
	}
	request.Body = http.MaxBytesReader(response, request.Body, maxRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			return domain.NewError(domain.CodeValidation, "请求体超过 1 MiB")
		}
		return domain.NewError(domain.CodeValidation, "JSON 请求无效：%v", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return domain.NewError(domain.CodeValidation, "请求体只能包含一个 JSON 对象")
	}
	return nil
}
