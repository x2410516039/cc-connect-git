package clouddoc

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) GrantDrivePermission(ctx context.Context, req DriveGrantRequest) (*Response, error) {
	resourceType := strings.TrimSpace(req.ResourceType)
	q := url.Values{}
	q.Set("type", resourceType)
	q.Set("need_notification", fmt.Sprintf("%t", req.NeedNotification))

	body := map[string]any{
		"member_type": "openid",
		"member_id":   strings.TrimSpace(req.OpenID),
		"perm":        strings.TrimSpace(req.Perm),
		"type":        "user",
	}
	if resourceType == "wiki" {
		body["perm_type"] = "container"
	}

	path := fmt.Sprintf("/open-apis/drive/v1/permissions/%s/members", url.PathEscape(strings.TrimSpace(req.Token)))
	return c.Call(ctx, http.MethodPost, path, q, body)
}
