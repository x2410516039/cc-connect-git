package clouddoc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCreateDocumentRequestPathAndPayload(t *testing.T) {
	called := false
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/open-apis/docs_ai/v1/documents" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tenant-token" {
			t.Errorf("authorization = %q", got)
		}
		assertJSONEqual(t, readBodyMap(t, r), map[string]any{
			"title":           "MVP Title",
			"format":          "markdown",
			"content":         "# hello",
			"parent_token":    "fld_token",
			"parent_position": "first",
		})
		writeEnvelope(t, w, map[string]any{
			"document": map[string]any{"document_id": "doc_created"},
		}, "doc-log")
	})

	res, err := client.CreateDocument(context.Background(), CreateDocumentRequest{
		Title:          " MVP Title ",
		Format:         " markdown ",
		Content:        "# hello",
		ParentToken:    " fld_token ",
		ParentPosition: " first ",
	})
	if err != nil {
		t.Fatalf("CreateDocument returned error: %v", err)
	}
	if !called {
		t.Fatal("document endpoint was not called")
	}
	doc := res.Data["document"].(map[string]any)
	if got := doc["url"]; got != "https://www.feishu.cn/docx/doc_created" {
		t.Fatalf("document url = %v", got)
	}
}

func TestFetchDocumentRequestPathAndPayload(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/open-apis/docs_ai/v1/documents/doc_token/fetch" {
			t.Errorf("path = %s", r.URL.Path)
		}
		assertJSONEqual(t, readBodyMap(t, r), map[string]any{
			"format":      "xml",
			"revision_id": 7,
			"export_option": map[string]any{
				"export_block_id":        true,
				"export_style_attrs":     true,
				"export_cite_extra_data": true,
			},
			"read_option": map[string]any{
				"read_mode":      "section",
				"start_block_id": "blk_start",
				"context_before": "2",
				"context_after":  "3",
				"max_depth":      "4",
			},
		})
		writeEnvelope(t, w, map[string]any{"content": "<doc/>"}, "fetch-log")
	})

	_, err := client.FetchDocument(context.Background(), FetchDocumentRequest{
		DocumentID:    " doc_token ",
		Format:        " xml ",
		Detail:        "full",
		RevisionID:    7,
		Scope:         "section",
		StartBlockID:  " blk_start ",
		ContextBefore: 2,
		ContextAfter:  3,
		MaxDepth:      4,
	})
	if err != nil {
		t.Fatalf("FetchDocument returned error: %v", err)
	}
}

func TestUpdateDocumentRequestConstruction(t *testing.T) {
	tests := []struct {
		name string
		req  UpdateDocumentRequest
		want map[string]any
	}{
		{
			name: "append maps to block insert after document end",
			req:  UpdateDocumentRequest{DocumentID: "doc_token", Command: "append", Format: "markdown", Content: "tail"},
			want: map[string]any{
				"format":   "markdown",
				"command":  "block_insert_after",
				"content":  "tail",
				"block_id": "-1",
			},
		},
		{
			name: "overwrite",
			req:  UpdateDocumentRequest{DocumentID: "doc_token", Command: "overwrite", Format: "xml", Content: "<doc/>", RevisionID: 11},
			want: map[string]any{
				"format":      "xml",
				"command":     "overwrite",
				"content":     "<doc/>",
				"revision_id": 11,
			},
		},
		{
			name: "str replace",
			req:  UpdateDocumentRequest{DocumentID: "doc_token", Command: "str_replace", Format: "markdown", Content: "new", Pattern: "old", MaxReplacements: 2},
			want: map[string]any{
				"format":           "markdown",
				"command":          "str_replace",
				"content":          "new",
				"pattern":          "old",
				"max_replacements": 2,
			},
		},
		{
			name: "block insert after",
			req:  UpdateDocumentRequest{DocumentID: "doc_token", Command: "block_insert_after", Format: "markdown", Content: "child", BlockID: "blk_1"},
			want: map[string]any{
				"format":   "markdown",
				"command":  "block_insert_after",
				"content":  "child",
				"block_id": "blk_1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("method = %s, want PUT", r.Method)
				}
				if r.URL.Path != "/open-apis/docs_ai/v1/documents/doc_token" {
					t.Errorf("path = %s", r.URL.Path)
				}
				assertJSONEqual(t, readBodyMap(t, r), tt.want)
				writeEnvelope(t, w, map[string]any{"ok": true}, "update-log")
			})
			if _, err := client.UpdateDocument(context.Background(), tt.req); err != nil {
				t.Fatalf("UpdateDocument returned error: %v", err)
			}
		})
	}
}

func TestGrantDrivePermissionRequestPathQueryAndPayload(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/open-apis/drive/v1/permissions/wiki_token/members" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("type"); got != "wiki" {
			t.Errorf("query type = %q", got)
		}
		if got := r.URL.Query().Get("need_notification"); got != "true" {
			t.Errorf("query need_notification = %q", got)
		}
		assertJSONEqual(t, readBodyMap(t, r), map[string]any{
			"member_type": "openid",
			"member_id":   "ou_user",
			"perm":        "full_access",
			"type":        "user",
			"perm_type":   "container",
		})
		writeEnvelope(t, w, map[string]any{"member": map[string]any{"member_id": "ou_user"}}, "drive-log")
	})

	_, err := client.GrantDrivePermission(context.Background(), DriveGrantRequest{
		Token:            " wiki_token ",
		ResourceType:     " wiki ",
		OpenID:           " ou_user ",
		Perm:             " full_access ",
		NeedNotification: true,
	})
	if err != nil {
		t.Fatalf("GrantDrivePermission returned error: %v", err)
	}
}

func TestAPIErrorIncludesCodeStatusAndLogID(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Tt-Logid", "header-log")
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(`{"code":124,"msg":"bad request","data":{},"log_id":"body-log"}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	})

	_, err := client.FetchDocument(context.Background(), FetchDocumentRequest{DocumentID: "doc_token", Format: "xml"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want APIError", err)
	}
	if apiErr.Type != "http_error" || apiErr.Code != 124 || apiErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("APIError = %+v", apiErr)
	}
	if apiErr.APIPath != "/open-apis/docs_ai/v1/documents/doc_token/fetch" {
		t.Fatalf("APIPath = %q", apiErr.APIPath)
	}
	if apiErr.LogID != "body-log" {
		t.Fatalf("LogID = %q, want body-log", apiErr.LogID)
	}
}

func TestTenantTokenAPIErrorIncludesSafeAPIPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tenantTokenAPIPath {
			t.Errorf("auth path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"code":99991663,"msg":"invalid app credentials","log_id":"auth-log"}`)); err != nil {
			t.Errorf("write auth response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(Config{
		Platform:  "feishu",
		BaseURL:   server.URL,
		AppID:     "app-id",
		AppSecret: "app-secret",
		HTTP:      server.Client(),
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	err = client.RefreshTenantToken(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want APIError", err)
	}
	if apiErr.APIPath != tenantTokenAPIPath {
		t.Fatalf("APIPath = %q", apiErr.APIPath)
	}
	if apiErr.Code != 99991663 || apiErr.LogID != "auth-log" {
		t.Fatalf("APIError = %+v", apiErr)
	}
}

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open-apis/auth/v3/tenant_access_token/internal" {
			if r.Method != http.MethodPost {
				t.Errorf("auth method = %s, want POST", r.Method)
			}
			assertJSONEqual(t, readBodyMap(t, r), map[string]any{
				"app_id":     "app-id",
				"app_secret": "app-secret",
			})
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(`{"code":0,"tenant_access_token":"tenant-token","log_id":"auth-log"}`)); err != nil {
				t.Errorf("write auth response: %v", err)
			}
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(Config{
		Platform:  "feishu",
		BaseURL:   server.URL,
		AppID:     "app-id",
		AppSecret: "app-secret",
		HTTP:      server.Client(),
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	return client
}

func readBodyMap(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return body
}

func writeEnvelope(t *testing.T, w http.ResponseWriter, data map[string]any, logID string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"code":   0,
		"msg":    "ok",
		"data":   data,
		"log_id": logID,
	}); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func assertJSONEqual(t *testing.T, got, want any) {
	t.Helper()
	got = normalizeJSON(t, got)
	want = normalizeJSON(t, want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func normalizeJSON(t *testing.T, value any) any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	return out
}
