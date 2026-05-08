package clouddoc

import (
	"errors"
	"fmt"
)

type APIError struct {
	Type       string
	APIPath    string
	Message    string
	Code       int
	LogID      string
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code != 0 {
		return fmt.Sprintf("%s: code=%d msg=%s", e.Type, e.Code, e.Message)
	}
	if e.HTTPStatus != 0 {
		return fmt.Sprintf("%s: status=%d msg=%s", e.Type, e.HTTPStatus, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func IsInvalidTenantToken(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr == nil {
		return false
	}
	if apiErr.Code == 99991663 {
		return true
	}
	return containsFold(apiErr.Message, "invalid access token") ||
		containsFold(apiErr.Message, "tenant access token")
}
