package clouddoc

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (c *Client) CreateDocument(ctx context.Context, req CreateDocumentRequest) (*Response, error) {
	body := map[string]any{
		"title":   strings.TrimSpace(req.Title),
		"format":  strings.TrimSpace(req.Format),
		"content": req.Content,
	}
	if v := strings.TrimSpace(req.ParentToken); v != "" {
		body["parent_token"] = v
	}
	if v := strings.TrimSpace(req.ParentPosition); v != "" {
		body["parent_position"] = v
	}
	res, err := c.Call(ctx, http.MethodPost, "/open-apis/docs_ai/v1/documents", nil, body)
	if err != nil {
		return nil, err
	}
	c.ensureDocumentURL(res)
	return res, nil
}

func (c *Client) FetchDocument(ctx context.Context, req FetchDocumentRequest) (*Response, error) {
	body := map[string]any{
		"format": strings.TrimSpace(req.Format),
	}
	if req.RevisionID > 0 {
		body["revision_id"] = req.RevisionID
	}
	switch strings.TrimSpace(req.Detail) {
	case "", "simple":
		body["export_option"] = map[string]any{
			"export_block_id":        false,
			"export_style_attrs":     false,
			"export_cite_extra_data": false,
		}
	case "with-ids":
		body["export_option"] = map[string]any{"export_block_id": true}
	case "full":
		body["export_option"] = map[string]any{
			"export_block_id":        true,
			"export_style_attrs":     true,
			"export_cite_extra_data": true,
		}
	}
	if ro := buildReadOption(req); ro != nil {
		body["read_option"] = ro
	}
	path := fmt.Sprintf("/open-apis/docs_ai/v1/documents/%s/fetch", url.PathEscape(strings.TrimSpace(req.DocumentID)))
	return c.Call(ctx, http.MethodPost, path, nil, body)
}

func (c *Client) UpdateDocument(ctx context.Context, req UpdateDocumentRequest) (*Response, error) {
	cmd := strings.TrimSpace(req.Command)
	blockID := strings.TrimSpace(req.BlockID)
	if cmd == "append" {
		cmd = "block_insert_after"
		blockID = "-1"
	}

	body := map[string]any{
		"format":  strings.TrimSpace(req.Format),
		"command": cmd,
	}
	if req.RevisionID > 0 {
		body["revision_id"] = req.RevisionID
	}
	if req.Content != "" {
		body["content"] = req.Content
	}
	if v := strings.TrimSpace(req.Pattern); v != "" {
		body["pattern"] = v
	}
	if blockID != "" {
		body["block_id"] = blockID
	}
	if req.MaxReplacements > 0 && strings.TrimSpace(req.Command) == "str_replace" {
		body["max_replacements"] = req.MaxReplacements
	}
	path := fmt.Sprintf("/open-apis/docs_ai/v1/documents/%s", url.PathEscape(strings.TrimSpace(req.DocumentID)))
	return c.Call(ctx, http.MethodPut, path, nil, body)
}

func buildReadOption(req FetchDocumentRequest) map[string]any {
	mode := strings.TrimSpace(req.Scope)
	if mode == "" || mode == "full" {
		return nil
	}
	ro := map[string]any{"read_mode": mode}
	if v := strings.TrimSpace(req.StartBlockID); v != "" {
		ro["start_block_id"] = v
	}
	if v := strings.TrimSpace(req.EndBlockID); v != "" {
		ro["end_block_id"] = v
	}
	if v := strings.TrimSpace(req.Keyword); v != "" {
		ro["keyword"] = v
	}
	if req.ContextBefore > 0 {
		ro["context_before"] = strconv.Itoa(req.ContextBefore)
	}
	if req.ContextAfter > 0 {
		ro["context_after"] = strconv.Itoa(req.ContextAfter)
	}
	if req.MaxDepth >= 0 {
		ro["max_depth"] = strconv.Itoa(req.MaxDepth)
	}
	return ro
}

func (c *Client) ensureDocumentURL(res *Response) {
	if res == nil || res.Data == nil {
		return
	}
	doc, _ := res.Data["document"].(map[string]any)
	if doc == nil {
		return
	}
	if v, _ := doc["url"].(string); strings.TrimSpace(v) != "" {
		return
	}
	docID, _ := doc["document_id"].(string)
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return
	}
	host := "https://www.feishu.cn"
	if strings.EqualFold(c.platform, "lark") {
		host = "https://www.larksuite.com"
	}
	doc["url"] = host + "/docx/" + docID
}
