package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenhg5/cc-connect/config"
	"github.com/chenhg5/cc-connect/platform/feishu/clouddoc"
)

func TestFeishuDocContentInputValidation(t *testing.T) {
	t.Run("rejects multiple input sources", func(t *testing.T) {
		fs, content, contentFile, stdin := newContentFlagSet()
		if err := parseFeishuDocFlags(fs, []string{"--content", "inline", "--stdin"}); err != nil {
			t.Fatalf("parseFeishuDocFlags returned error: %v", err)
		}
		_, err := readFeishuDocContent(fs, *content, *contentFile, *stdin, true)
		assertErrContains(t, err, "use only one of --content, --content-file, or --stdin")
	})

	t.Run("rejects empty required content", func(t *testing.T) {
		fs, content, contentFile, stdin := newContentFlagSet()
		if err := parseFeishuDocFlags(fs, []string{"--content", "   "}); err != nil {
			t.Fatalf("parseFeishuDocFlags returned error: %v", err)
		}
		_, err := readFeishuDocContent(fs, *content, *contentFile, *stdin, true)
		assertErrContains(t, err, "content cannot be empty")
	})

	t.Run("reads content file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "body.md")
		if err := os.WriteFile(path, []byte("# file\n"), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		fs, content, contentFile, stdin := newContentFlagSet()
		if err := parseFeishuDocFlags(fs, []string{"--content-file", path}); err != nil {
			t.Fatalf("parseFeishuDocFlags returned error: %v", err)
		}
		got, err := readFeishuDocContent(fs, *content, *contentFile, *stdin, true)
		if err != nil {
			t.Fatalf("readFeishuDocContent returned error: %v", err)
		}
		if got != "# file\n" {
			t.Fatalf("content = %q", got)
		}
	})
}

func TestFeishuDocValidationHelpers(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr string
	}{
		{
			name:    "create requires title",
			err:     validateCreateDocumentFlags("", "markdown", "", ""),
			wantErr: "--title is required",
		},
		{
			name:    "create rejects both parent placements",
			err:     validateCreateDocumentFlags("title", "markdown", "parent", "first"),
			wantErr: "--parent-token and --parent-position are mutually exclusive",
		},
		{
			name:    "overwrite requires explicit yes",
			err:     validateUpdateFlags("overwrite", "markdown", "", "", 1, false),
			wantErr: "--command overwrite requires --yes",
		},
		{
			name:    "str replace requires pattern",
			err:     validateUpdateFlags("str_replace", "markdown", "", "", 1, true),
			wantErr: "--command str_replace requires --pattern",
		},
		{
			name:    "block insert requires block id",
			err:     validateUpdateFlags("block_insert_after", "markdown", "", "", 1, true),
			wantErr: "--command block_insert_after requires --block-id",
		},
		{
			name:    "drive grant requires token",
			err:     validateDriveGrantFlags("", "docx", "ou_user", "full_access"),
			wantErr: "--token is required",
		},
		{
			name:    "drive grant requires open id",
			err:     validateDriveGrantFlags("doc_token", "docx", "", "full_access"),
			wantErr: "--open-id is required",
		},
		{
			name:    "fetch range requires block boundary",
			err:     validateFetchFlags("xml", "with-ids", "range", "", "", "", 0, 0, -1),
			wantErr: "range scope requires --start-block-id or --end-block-id",
		},
		{
			name:    "text cannot export ids",
			err:     validateFetchFlags("text", "with-ids", "full", "", "", "", 0, 0, -1),
			wantErr: "--detail with-ids is only supported with --format xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertErrContains(t, tt.err, tt.wantErr)
		})
	}

	if err := validateUpdateFlags("overwrite", "markdown", "", "", 1, true); err != nil {
		t.Fatalf("validateUpdateFlags overwrite with yes returned error: %v", err)
	}
}

func TestFeishuDocResourceRefParsing(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		defaultKind string
		want        resourceRef
		wantErr     string
	}{
		{
			name:        "docx URL",
			input:       " https://example.feishu.cn/docx/doc_token_123?from=copy ",
			defaultKind: "docx",
			want:        resourceRef{Kind: "docx", Token: "doc_token_123"},
		},
		{
			name:        "wiki URL",
			input:       "https://example.feishu.cn/wiki/wiki_token_123?table=tbl",
			defaultKind: "docx",
			want:        resourceRef{Kind: "wiki", Token: "wiki_token_123"},
		},
		{
			name:        "drive folder URL",
			input:       "https://example.larksuite.com/drive/folder/folder_token_123",
			defaultKind: "docx",
			want:        resourceRef{Kind: "folder", Token: "folder_token_123"},
		},
		{
			name:        "raw token uses default kind",
			input:       "raw_doc_token",
			defaultKind: "docx",
			want:        resourceRef{Kind: "docx", Token: "raw_doc_token"},
		},
		{
			name:        "empty token",
			input:       " ",
			defaultKind: "docx",
			wantErr:     "token cannot be empty",
		},
		{
			name:        "unsupported URL",
			input:       "https://example.com/not-a-doc/token",
			defaultKind: "docx",
			wantErr:     "unsupported URL",
		},
		{
			name:        "slash in raw token",
			input:       "not/a/raw/token",
			defaultKind: "docx",
			wantErr:     "unsupported token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseResourceRef(tt.input, tt.defaultKind)
			if tt.wantErr != "" {
				assertErrContains(t, err, tt.wantErr)
				return
			}
			if err != nil {
				t.Fatalf("parseResourceRef returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("ref = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestResolveDocFlagRefValidation(t *testing.T) {
	_, err := resolveDocFlagRef("", "")
	assertErrContains(t, err, "--doc or --document-id is required")

	_, err = resolveDocFlagRef("doc_a", "doc_b")
	assertErrContains(t, err, "use either --doc or --document-id")

	ref, err := resolveDocFlagRef("", "https://example.feishu.cn/docx/doc_token")
	if err != nil {
		t.Fatalf("resolveDocFlagRef returned error: %v", err)
	}
	if ref != (resourceRef{Kind: "docx", Token: "doc_token"}) {
		t.Fatalf("ref = %+v", ref)
	}
}

func TestFeishuDocConfigProjectPlatformResolution(t *testing.T) {
	configPath := writeFeishuDocConfigFixture(t, `
[[projects]]
name = "alpha"

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/tmp/alpha"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
token = "tg-token"

[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "cli_alpha"
app_secret = "sec_alpha"
domain = "http://feishu.local"

[[projects.platforms]]
type = "lark"

[projects.platforms.options]
app_id = "cli_lark"
app_secret = "sec_lark"
base_url = "http://lark.local"

[[projects]]
name = "beta"

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/tmp/beta"

[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "cli_beta"
app_secret = "sec_beta"
`)
	patchConfigPath(t, configPath)
	t.Setenv("CC_PROJECT", "")

	_, err := resolveFeishuDocTarget("", 0)
	assertErrContains(t, err, "multiple projects found")

	t.Setenv("CC_PROJECT", "beta")
	target, err := resolveFeishuDocTarget("", 0)
	if err != nil {
		t.Fatalf("resolveFeishuDocTarget env project returned error: %v", err)
	}
	if target.ProjectName != "beta" || target.Platform.Type != "feishu" || target.PlatformPos != 0 {
		t.Fatalf("target = %+v", target)
	}

	client, target, err := newFeishuDocClient(&feishuDocCommonFlags{
		Config:        configPath,
		Project:       "alpha",
		PlatformIndex: 2,
	})
	if err != nil {
		t.Fatalf("newFeishuDocClient returned error: %v", err)
	}
	if target.ProjectName != "alpha" || target.Platform.Type != "lark" || target.PlatformPos != 2 {
		t.Fatalf("target = %+v", target)
	}
	if client.Platform() != "lark" || client.BaseURL() != "http://lark.local" {
		t.Fatalf("client platform/base = %q/%q", client.Platform(), client.BaseURL())
	}

	_, _, err = newFeishuDocClient(&feishuDocCommonFlags{
		Config:        configPath,
		Project:       "alpha",
		PlatformIndex: 3,
	})
	assertErrContains(t, err, "--platform-index 3 out of range")
}

func TestResolveFeishuDocConfigPathPrecedence(t *testing.T) {
	t.Setenv("CC_CONFIG_PATH", "env-config.toml")
	if got := resolveFeishuDocConfigPath("flag-config.toml"); got != "flag-config.toml" {
		t.Fatalf("flag config path = %q", got)
	}
	if got := resolveFeishuDocConfigPath(""); got != "env-config.toml" {
		t.Fatalf("env config path = %q", got)
	}
}

func TestBuildFeishuDocSuccessPayloadSurfacesStableFields(t *testing.T) {
	payload := buildFeishuDocSuccessPayload("wiki.create", &feishuDocTarget{
		ProjectName: "alpha",
		PlatformPos: 1,
	}, "feishu", &clouddoc.Response{
		LogID: "log-123",
		Data: map[string]any{
			"node": map[string]any{
				"node_token": "wikcn_token",
				"obj_token":  "docx_token",
				"obj_type":   "docx",
				"url":        "https://example.feishu.cn/wiki/wikcn_token",
			},
			"permission_grants": []any{map[string]any{"member_id": "ou_user"}},
		},
	})

	assertPayloadValue(t, payload, "ok", true)
	assertPayloadValue(t, payload, "operation", "wiki.create")
	assertPayloadValue(t, payload, "project", "alpha")
	assertPayloadValue(t, payload, "platform", "feishu")
	assertPayloadValue(t, payload, "platform_index", 2)
	assertPayloadValue(t, payload, "log_id", "log-123")
	assertPayloadValue(t, payload, "wiki_node_token", "wikcn_token")
	assertPayloadValue(t, payload, "obj_token", "docx_token")
	assertPayloadValue(t, payload, "obj_type", "docx")
	assertPayloadValue(t, payload, "url", "https://example.feishu.cn/wiki/wikcn_token")
	if _, ok := payload["data"].(map[string]any); !ok {
		t.Fatalf("data = %#v, want raw response map", payload["data"])
	}
	if _, ok := payload["permission_grants"]; !ok {
		t.Fatal("permission_grants was not surfaced")
	}
}

func TestBuildFeishuDocErrorPayloadSurfacesOperationAndAPIErrorFields(t *testing.T) {
	payload := buildFeishuDocErrorPayload("doc.update", &clouddoc.APIError{
		Type:       "api_error",
		APIPath:    "/open-apis/docs_ai/v1/documents/doc_token",
		Message:    "permission denied",
		Code:       99991672,
		LogID:      "log-456",
		HTTPStatus: 403,
	})

	assertPayloadValue(t, payload, "ok", false)
	assertPayloadValue(t, payload, "operation", "doc.update")
	assertPayloadValue(t, payload, "api_path", "/open-apis/docs_ai/v1/documents/doc_token")
	assertPayloadValue(t, payload, "code", 99991672)
	assertPayloadValue(t, payload, "message", "permission denied")
	assertPayloadValue(t, payload, "log_id", "log-456")
	assertPayloadValue(t, payload, "http_status", 403)
	nested, _ := payload["error"].(map[string]any)
	assertPayloadValue(t, nested, "api_path", "/open-apis/docs_ai/v1/documents/doc_token")
}

func TestBuildFeishuDocErrorPayloadSurfacesValidationMessage(t *testing.T) {
	payload := buildFeishuDocErrorPayload("doc.create", cliValidationError("--title is required"))

	assertPayloadValue(t, payload, "ok", false)
	assertPayloadValue(t, payload, "operation", "doc.create")
	assertPayloadValue(t, payload, "message", "--title is required")
	assertPayloadValue(t, payload, "type", "validation_error")
}

func newContentFlagSet() (*flag.FlagSet, *string, *string, *bool) {
	fs := flag.NewFlagSet("content-test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	content := fs.String("content", "", "")
	contentFile := fs.String("content-file", "", "")
	stdin := fs.Bool("stdin", false, "")
	return fs, content, contentFile, stdin
}

func writeFeishuDocConfigFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	return path
}

func patchConfigPath(t *testing.T, path string) {
	t.Helper()
	prev := config.ConfigPath
	config.ConfigPath = path
	t.Cleanup(func() {
		config.ConfigPath = prev
	})
}

func assertErrContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want containing %q", err.Error(), want)
	}
}

func assertPayloadValue(t *testing.T, payload map[string]any, key string, want any) {
	t.Helper()
	if got := payload[key]; got != want {
		t.Fatalf("%s = %#v, want %#v", key, got, want)
	}
}
