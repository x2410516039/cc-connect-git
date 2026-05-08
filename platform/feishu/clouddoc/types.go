package clouddoc

import (
	"encoding/json"
	"net/http"
)

type Config struct {
	Platform  string
	BaseURL   string
	AppID     string
	AppSecret string
	HTTP      *http.Client
}

type Response struct {
	Data       map[string]any
	LogID      string
	HTTPStatus int
}

type apiEnvelope struct {
	Code  int             `json:"code"`
	Msg   string          `json:"msg"`
	Data  json.RawMessage `json:"data"`
	LogID string          `json:"log_id"`
}

type tenantTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	LogID             string `json:"log_id"`
}

type CreateDocumentRequest struct {
	Title          string
	Format         string
	Content        string
	ParentToken    string
	ParentPosition string
}

type FetchDocumentRequest struct {
	DocumentID    string
	Format        string
	Detail        string
	RevisionID    int
	Scope         string
	StartBlockID  string
	EndBlockID    string
	Keyword       string
	ContextBefore int
	ContextAfter  int
	MaxDepth      int
}

type UpdateDocumentRequest struct {
	DocumentID      string
	Command         string
	Format          string
	Content         string
	Pattern         string
	BlockID         string
	RevisionID      int
	MaxReplacements int
}

type WikiCreateRequest struct {
	SpaceID         string
	ParentNodeToken string
	Title           string
	NodeType        string
	ObjType         string
	OriginNodeToken string
}

type DriveGrantRequest struct {
	Token            string
	ResourceType     string
	OpenID           string
	Perm             string
	NeedNotification bool
}
