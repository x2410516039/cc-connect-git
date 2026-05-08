package clouddoc

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) ResolveWikiNode(ctx context.Context, token string) (*Response, error) {
	q := url.Values{}
	q.Set("token", strings.TrimSpace(token))
	return c.Call(ctx, http.MethodGet, "/open-apis/wiki/v2/spaces/get_node", q, nil)
}

func (c *Client) GetWikiSpace(ctx context.Context, spaceID string) (*Response, error) {
	path := fmt.Sprintf("/open-apis/wiki/v2/spaces/%s", url.PathEscape(strings.TrimSpace(spaceID)))
	return c.Call(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) CreateWikiNode(ctx context.Context, req WikiCreateRequest) (*Response, error) {
	body := map[string]any{
		"node_type": strings.TrimSpace(req.NodeType),
		"obj_type":  strings.TrimSpace(req.ObjType),
	}
	if v := strings.TrimSpace(req.Title); v != "" {
		body["title"] = v
	}
	if v := strings.TrimSpace(req.ParentNodeToken); v != "" {
		body["parent_node_token"] = v
	}
	if v := strings.TrimSpace(req.OriginNodeToken); v != "" {
		body["origin_node_token"] = v
	}
	path := fmt.Sprintf("/open-apis/wiki/v2/spaces/%s/nodes", url.PathEscape(strings.TrimSpace(req.SpaceID)))
	res, err := c.Call(ctx, http.MethodPost, path, nil, body)
	if err != nil {
		return nil, err
	}
	c.ensureWikiURL(res)
	return res, nil
}

func (c *Client) ensureWikiURL(res *Response) {
	if res == nil || res.Data == nil {
		return
	}
	node, _ := res.Data["node"].(map[string]any)
	if node == nil {
		return
	}
	if v, _ := node["url"].(string); strings.TrimSpace(v) != "" {
		return
	}
	token, _ := node["node_token"].(string)
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	host := "https://www.feishu.cn"
	if strings.EqualFold(c.platform, "lark") {
		host = "https://www.larksuite.com"
	}
	node["url"] = host + "/wiki/" + token
}
