package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/chenhg5/cc-connect/config"
	"github.com/chenhg5/cc-connect/platform/feishu/clouddoc"
)

type feishuDocCommonFlags struct {
	Config        string
	Project       string
	PlatformIndex int
	Quiet         bool
}

type feishuDocTarget struct {
	ProjectName string
	Platform    config.PlatformConfig
	PlatformPos int
}

type feishuDocCLIError struct {
	Type    string
	Message string
}

func (e *feishuDocCLIError) Error() string { return e.Message }

func runFeishuDoc(args []string) {
	if len(args) == 0 {
		printFeishuDocUsage()
		return
	}
	switch args[0] {
	case "create":
		runFeishuDocCreate(args[1:])
	case "fetch":
		runFeishuDocFetch(args[1:])
	case "update":
		runFeishuDocUpdate(args[1:])
	case "help", "--help", "-h":
		printFeishuDocUsage()
	default:
		feishuDocExit("doc.unknown", cliValidationError("unknown feishu doc subcommand: %s", args[0]), false)
	}
}

func runFeishuWiki(args []string) {
	if len(args) == 0 {
		printFeishuWikiUsage()
		return
	}
	switch args[0] {
	case "create":
		runFeishuWikiCreate(args[1:])
	case "resolve":
		runFeishuWikiResolve(args[1:])
	case "help", "--help", "-h":
		printFeishuWikiUsage()
	default:
		feishuDocExit("wiki.unknown", cliValidationError("unknown feishu wiki subcommand: %s", args[0]), false)
	}
}

func runFeishuDrive(args []string) {
	if len(args) == 0 {
		printFeishuDriveUsage()
		return
	}
	switch args[0] {
	case "grant":
		runFeishuDriveGrant(args[1:])
	case "help", "--help", "-h":
		printFeishuDriveUsage()
	default:
		feishuDocExit("drive.unknown", cliValidationError("unknown feishu drive subcommand: %s", args[0]), false)
	}
}

func runFeishuDocCreate(args []string) {
	const operation = "doc.create"
	fs, common := newFeishuDocFlagSet("feishu doc create")
	title := fs.String("title", "", "document title")
	format := fs.String("format", "markdown", "content format: markdown or xml")
	content := fs.String("content", "", "document content")
	contentFile := fs.String("content-file", "", "read document content from file")
	stdin := fs.Bool("stdin", false, "read document content from stdin")
	parentToken := fs.String("parent-token", "", "parent folder or wiki-node token")
	parentPosition := fs.String("parent-position", "", "parent position")
	if err := parseFeishuDocFlags(fs, args); err != nil {
		feishuDocExit(operation, err, false)
	}
	body, err := readFeishuDocContent(fs, *content, *contentFile, *stdin, true)
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	docFormat := strings.ToLower(strings.TrimSpace(*format))
	if err := validateCreateDocumentFlags(*title, docFormat, *parentToken, *parentPosition); err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}

	client, target, err := newFeishuDocClient(common)
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	res, err := client.CreateDocument(context.Background(), clouddoc.CreateDocumentRequest{
		Title:          *title,
		Format:         docFormat,
		Content:        body,
		ParentToken:    *parentToken,
		ParentPosition: *parentPosition,
	})
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	writeFeishuDocSuccess(common.Quiet, operation, target, client.Platform(), res)
}

func runFeishuDocFetch(args []string) {
	const operation = "doc.fetch"
	fs, common := newFeishuDocFlagSet("feishu doc fetch")
	doc := fs.String("doc", "", "document URL or token")
	documentID := fs.String("document-id", "", "document URL or token")
	format := fs.String("format", "xml", "content format: xml, markdown, or text")
	detail := fs.String("detail", "with-ids", "export detail: simple, with-ids, or full")
	revisionID := fs.Int("revision-id", -1, "document revision id")
	scope := fs.String("scope", "full", "read scope: full, outline, range, keyword, or section")
	startBlockID := fs.String("start-block-id", "", "range/section start block id")
	endBlockID := fs.String("end-block-id", "", "range end block id")
	keyword := fs.String("keyword", "", "keyword scope query")
	contextBefore := fs.Int("context-before", 0, "sibling blocks before match")
	contextAfter := fs.Int("context-after", 0, "sibling blocks after match")
	maxDepth := fs.Int("max-depth", -1, "subtree depth; -1 means default/unlimited")
	if err := parseFeishuDocFlags(fs, args); err != nil {
		feishuDocExit(operation, err, false)
	}
	ref, err := resolveDocFlagRef(*doc, *documentID)
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	docFormat := strings.ToLower(strings.TrimSpace(*format))
	detailLevel := strings.ToLower(strings.TrimSpace(*detail))
	readScope := strings.ToLower(strings.TrimSpace(*scope))
	if err := validateDocFormat(docFormat, true); err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	if err := validateFetchFlags(docFormat, detailLevel, readScope, *startBlockID, *endBlockID, *keyword, *contextBefore, *contextAfter, *maxDepth); err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}

	client, target, err := newFeishuDocClient(common)
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	res, err := client.FetchDocument(context.Background(), clouddoc.FetchDocumentRequest{
		DocumentID:    ref.Token,
		Format:        docFormat,
		Detail:        detailLevel,
		RevisionID:    *revisionID,
		Scope:         readScope,
		StartBlockID:  *startBlockID,
		EndBlockID:    *endBlockID,
		Keyword:       *keyword,
		ContextBefore: *contextBefore,
		ContextAfter:  *contextAfter,
		MaxDepth:      *maxDepth,
	})
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	writeFeishuDocSuccess(common.Quiet, operation, target, client.Platform(), res)
}

func runFeishuDocUpdate(args []string) {
	const operation = "doc.update"
	fs, common := newFeishuDocFlagSet("feishu doc update")
	doc := fs.String("doc", "", "document URL or token")
	documentID := fs.String("document-id", "", "document URL or token")
	command := fs.String("command", "", "operation: append, overwrite, str_replace, or block_insert_after")
	format := fs.String("format", "markdown", "content format: markdown or xml")
	content := fs.String("content", "", "document content")
	contentFile := fs.String("content-file", "", "read document content from file")
	stdin := fs.Bool("stdin", false, "read document content from stdin")
	pattern := fs.String("pattern", "", "pattern for str_replace")
	blockID := fs.String("block-id", "", "target block id for block_insert_after")
	revisionID := fs.Int("revision-id", -1, "base revision id")
	maxReplacements := fs.Int("max-replacements", 1, "maximum replacements for str_replace")
	yes := fs.Bool("yes", false, "confirm overwrite")
	if err := parseFeishuDocFlags(fs, args); err != nil {
		feishuDocExit(operation, err, false)
	}
	ref, err := resolveDocFlagRef(*doc, *documentID)
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	cmd := strings.ToLower(strings.TrimSpace(*command))
	docFormat := strings.ToLower(strings.TrimSpace(*format))
	if err := validateUpdateFlags(cmd, docFormat, *pattern, *blockID, *maxReplacements, *yes); err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	body, err := readFeishuDocContent(fs, *content, *contentFile, *stdin, true)
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}

	client, target, err := newFeishuDocClient(common)
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	res, err := client.UpdateDocument(context.Background(), clouddoc.UpdateDocumentRequest{
		DocumentID:      ref.Token,
		Command:         cmd,
		Format:          docFormat,
		Content:         body,
		Pattern:         *pattern,
		BlockID:         *blockID,
		RevisionID:      *revisionID,
		MaxReplacements: *maxReplacements,
	})
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	writeFeishuDocSuccess(common.Quiet, operation, target, client.Platform(), res)
}

func runFeishuWikiResolve(args []string) {
	const operation = "wiki.resolve"
	fs, common := newFeishuDocFlagSet("feishu wiki resolve")
	token := fs.String("token", "", "wiki node URL or token")
	if err := parseFeishuDocFlags(fs, args); err != nil {
		feishuDocExit(operation, err, false)
	}
	ref, err := parseResourceRef(*token, "wiki")
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	client, target, err := newFeishuDocClient(common)
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	res, err := client.ResolveWikiNode(context.Background(), ref.Token)
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	writeFeishuDocSuccess(common.Quiet, operation, target, client.Platform(), res)
}

func runFeishuWikiCreate(args []string) {
	const operation = "wiki.create"
	fs, common := newFeishuDocFlagSet("feishu wiki create")
	spaceID := fs.String("space-id", "", "target wiki space id")
	parentNodeToken := fs.String("parent-node-token", "", "parent wiki node URL or token")
	title := fs.String("title", "", "wiki node title")
	nodeType := fs.String("node-type", "origin", "node type: origin or shortcut")
	objType := fs.String("obj-type", "docx", "object type: docx, sheet, mindnote, bitable, or slides")
	originNodeToken := fs.String("origin-node-token", "", "origin node URL or token for shortcut nodes")
	if err := parseFeishuDocFlags(fs, args); err != nil {
		feishuDocExit(operation, err, false)
	}
	if strings.TrimSpace(*title) == "" {
		feishuDocExit(operation, cliValidationError("--title is required"), common.Quiet)
	}
	parentRef, err := optionalResourceRef(*parentNodeToken, "wiki")
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	originRef, err := optionalResourceRef(*originNodeToken, "wiki")
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	nodeKind := strings.ToLower(strings.TrimSpace(*nodeType))
	objectKind := strings.ToLower(strings.TrimSpace(*objType))
	if err := validateWikiCreateFlags(*spaceID, parentRef.Token, nodeKind, objectKind, originRef.Token); err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}

	client, target, err := newFeishuDocClient(common)
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	resolvedSpace := strings.TrimSpace(*spaceID)
	if parentRef.Token != "" {
		parent, err := client.ResolveWikiNode(context.Background(), parentRef.Token)
		if err != nil {
			feishuDocExit(operation, err, common.Quiet)
		}
		parentSpace := nestedString(parent.Data, "node", "space_id")
		if parentSpace == "" {
			feishuDocExit(operation, cliAPIShapeError("wiki parent node lookup returned no space_id", parent.LogID), common.Quiet)
		}
		if resolvedSpace != "" && resolvedSpace != parentSpace {
			feishuDocExit(operation, cliValidationError("--space-id %q does not match parent node space %q", resolvedSpace, parentSpace), common.Quiet)
		}
		resolvedSpace = parentSpace
	}
	if resolvedSpace == "" {
		feishuDocExit(operation, cliValidationError("--space-id or --parent-node-token is required"), common.Quiet)
	}

	res, err := client.CreateWikiNode(context.Background(), clouddoc.WikiCreateRequest{
		SpaceID:         resolvedSpace,
		ParentNodeToken: parentRef.Token,
		Title:           *title,
		NodeType:        nodeKind,
		ObjType:         objectKind,
		OriginNodeToken: originRef.Token,
	})
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	if res.Data != nil {
		res.Data["resolved_space_id"] = resolvedSpace
	}
	writeFeishuDocSuccess(common.Quiet, operation, target, client.Platform(), res)
}

func runFeishuDriveGrant(args []string) {
	const operation = "drive.grant"
	fs, common := newFeishuDocFlagSet("feishu drive grant")
	token := fs.String("token", "", "resource URL or token")
	resourceType := fs.String("type", "", "resource type: docx, doc, wiki, sheet, bitable, file, folder, mindnote, or slides")
	openID := fs.String("open-id", "", "user open_id to grant")
	perm := fs.String("perm", "full_access", "permission level")
	needNotification := fs.Bool("need-notification", false, "send permission notification")
	if err := parseFeishuDocFlags(fs, args); err != nil {
		feishuDocExit(operation, err, false)
	}
	ref, err := parseResourceRef(*token, "docx")
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	typ := strings.ToLower(strings.TrimSpace(*resourceType))
	if typ == "" {
		typ = ref.Kind
	}
	if err := validateDriveGrantFlags(ref.Token, typ, *openID, *perm); err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	client, target, err := newFeishuDocClient(common)
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	res, err := client.GrantDrivePermission(context.Background(), clouddoc.DriveGrantRequest{
		Token:            ref.Token,
		ResourceType:     typ,
		OpenID:           *openID,
		Perm:             *perm,
		NeedNotification: *needNotification,
	})
	if err != nil {
		feishuDocExit(operation, err, common.Quiet)
	}
	writeFeishuDocSuccess(common.Quiet, operation, target, client.Platform(), res)
}

func newFeishuDocFlagSet(name string) (*flag.FlagSet, *feishuDocCommonFlags) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	common := &feishuDocCommonFlags{}
	fs.StringVar(&common.Config, "config", "", "path to config file")
	fs.StringVar(&common.Project, "project", "", "project name")
	fs.IntVar(&common.PlatformIndex, "platform-index", 0, "1-based index among feishu/lark platforms in the project (0 = first)")
	fs.BoolVar(&common.Quiet, "quiet", false, "suppress success output")
	return fs, common
}

func parseFeishuDocFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return cliValidationError("help is not available in JSON mode; use the command without subcommands for usage")
		}
		return cliValidationError("%s", err)
	}
	if fs.NArg() > 0 {
		return cliValidationError("unexpected argument: %s", fs.Arg(0))
	}
	return nil
}

func newFeishuDocClient(common *feishuDocCommonFlags) (*clouddoc.Client, *feishuDocTarget, error) {
	config.ConfigPath = resolveFeishuDocConfigPath(common.Config)
	target, err := resolveFeishuDocTarget(common.Project, common.PlatformIndex)
	if err != nil {
		return nil, nil, err
	}
	opts := target.Platform.Options
	appID := optionString(opts, "app_id")
	appSecret := optionString(opts, "app_secret")
	baseURL := optionString(opts, "domain")
	if baseURL == "" {
		baseURL = optionString(opts, "base_url")
	}
	platformType := strings.ToLower(strings.TrimSpace(target.Platform.Type))
	if baseURL == "" {
		baseURL = openFeishuBaseURL
		if platformType == "lark" {
			baseURL = openLarkBaseURL
		}
	}
	client, err := clouddoc.NewClient(clouddoc.Config{
		Platform:  platformType,
		BaseURL:   baseURL,
		AppID:     appID,
		AppSecret: appSecret,
	})
	if err != nil {
		return nil, nil, cliConfigError("%s", err)
	}
	return client, target, nil
}

func resolveFeishuDocConfigPath(flagValue string) string {
	if strings.TrimSpace(flagValue) != "" {
		return flagValue
	}
	if env := strings.TrimSpace(os.Getenv("CC_CONFIG_PATH")); env != "" {
		return env
	}
	return resolveConfigPath("")
}

func resolveFeishuDocTarget(projectFlag string, platformIndex int) (*feishuDocTarget, error) {
	if platformIndex < 0 {
		return nil, cliValidationError("--platform-index must be >= 0")
	}
	cfg, err := config.Load(config.ConfigPath)
	if err != nil {
		return nil, cliConfigError("%s", err)
	}
	projectName := strings.TrimSpace(projectFlag)
	if projectName == "" {
		projectName = strings.TrimSpace(os.Getenv("CC_PROJECT"))
	}
	if projectName == "" {
		switch len(cfg.Projects) {
		case 0:
			return nil, cliConfigError("no project found in config")
		case 1:
			projectName = cfg.Projects[0].Name
		default:
			names := make([]string, 0, len(cfg.Projects))
			for _, p := range cfg.Projects {
				names = append(names, p.Name)
			}
			sort.Strings(names)
			return nil, cliConfigError("multiple projects found, specify --project or CC_PROJECT (%s)", strings.Join(names, ", "))
		}
	}
	for _, project := range cfg.Projects {
		if project.Name != projectName {
			continue
		}
		var candidates []int
		for i, platform := range project.Platforms {
			t := strings.ToLower(strings.TrimSpace(platform.Type))
			if t == "feishu" || t == "lark" {
				candidates = append(candidates, i)
			}
		}
		if len(candidates) == 0 {
			return nil, cliConfigError("project %q has no feishu/lark platform", projectName)
		}
		pos := 0
		if platformIndex > 0 {
			pos = platformIndex - 1
		}
		if pos < 0 || pos >= len(candidates) {
			return nil, cliConfigError("--platform-index %d out of range: project %q has %d feishu/lark platform(s)", platformIndex, projectName, len(candidates))
		}
		abs := candidates[pos]
		return &feishuDocTarget{ProjectName: projectName, Platform: project.Platforms[abs], PlatformPos: abs}, nil
	}
	return nil, cliConfigError("project %q not found", projectName)
}

func optionString(opts map[string]any, key string) string {
	if opts == nil {
		return ""
	}
	v, _ := opts[key].(string)
	return strings.TrimSpace(v)
}

func readFeishuDocContent(fs *flag.FlagSet, content, contentFile string, stdin, required bool) (string, error) {
	contentSet := flagPassed(fs, "content")
	contentFileSet := flagPassed(fs, "content-file")
	count := 0
	if contentSet {
		count++
	}
	if contentFileSet {
		count++
	}
	if stdin {
		count++
	}
	if count > 1 {
		return "", cliValidationError("use only one of --content, --content-file, or --stdin")
	}
	var data []byte
	var err error
	switch {
	case contentSet:
		data = []byte(content)
	case contentFileSet:
		if strings.TrimSpace(contentFile) == "" {
			return "", cliValidationError("--content-file cannot be empty")
		}
		data, err = os.ReadFile(contentFile)
		if err != nil {
			return "", cliValidationError("read --content-file: %v", err)
		}
	case stdin:
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return "", cliValidationError("read --stdin: %v", err)
		}
	default:
		if required {
			return "", cliValidationError("--content, --content-file, or --stdin is required")
		}
		return "", nil
	}
	if required && strings.TrimSpace(string(data)) == "" {
		return "", cliValidationError("content cannot be empty")
	}
	return string(data), nil
}

func flagPassed(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

type resourceRef struct {
	Kind  string
	Token string
}

func resolveDocFlagRef(doc, documentID string) (resourceRef, error) {
	if strings.TrimSpace(doc) != "" && strings.TrimSpace(documentID) != "" {
		return resourceRef{}, cliValidationError("use either --doc or --document-id, not both")
	}
	raw := doc
	if strings.TrimSpace(raw) == "" {
		raw = documentID
	}
	if strings.TrimSpace(raw) == "" {
		return resourceRef{}, cliValidationError("--doc or --document-id is required")
	}
	return parseResourceRef(raw, "docx")
}

func optionalResourceRef(raw, defaultKind string) (resourceRef, error) {
	if strings.TrimSpace(raw) == "" {
		return resourceRef{}, nil
	}
	return parseResourceRef(raw, defaultKind)
}

func parseResourceRef(raw, defaultKind string) (resourceRef, error) {
	input := strings.TrimSpace(raw)
	if input == "" {
		return resourceRef{}, cliValidationError("token cannot be empty")
	}
	markers := []struct {
		Marker string
		Kind   string
	}{
		{"/wiki/", "wiki"},
		{"/docx/", "docx"},
		{"/doc/", "doc"},
		{"/sheets/", "sheet"},
		{"/base/", "bitable"},
		{"/bitable/", "bitable"},
		{"/file/", "file"},
		{"/drive/folder/", "folder"},
		{"/mindnote/", "mindnote"},
		{"/slides/", "slides"},
	}
	for _, marker := range markers {
		if token, ok := extractTokenAfterMarker(input, marker.Marker); ok {
			return resourceRef{Kind: marker.Kind, Token: token}, nil
		}
	}
	if strings.Contains(input, "://") {
		return resourceRef{}, cliValidationError("unsupported URL: expected a Feishu/Lark doc/wiki/drive resource URL")
	}
	if strings.ContainsAny(input, "/?#") {
		return resourceRef{}, cliValidationError("unsupported token %q: pass a raw token or a supported resource URL", input)
	}
	return resourceRef{Kind: defaultKind, Token: input}, nil
}

func extractTokenAfterMarker(raw, marker string) (string, bool) {
	idx := strings.Index(raw, marker)
	if idx < 0 {
		return "", false
	}
	token := raw[idx+len(marker):]
	if end := strings.IndexAny(token, "/?#"); end >= 0 {
		token = token[:end]
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	return token, true
}

func validateDocFormat(format string, allowText bool) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "xml", "markdown":
		return nil
	case "text":
		if allowText {
			return nil
		}
	}
	if allowText {
		return cliValidationError("--format must be xml, markdown, or text")
	}
	return cliValidationError("--format must be xml or markdown")
}

func validateCreateDocumentFlags(title, format, parentToken, parentPosition string) error {
	if strings.TrimSpace(title) == "" {
		return cliValidationError("--title is required")
	}
	if err := validateDocFormat(format, false); err != nil {
		return err
	}
	if strings.TrimSpace(parentToken) != "" && strings.TrimSpace(parentPosition) != "" {
		return cliValidationError("--parent-token and --parent-position are mutually exclusive")
	}
	return nil
}

func validateFetchFlags(format, detail, scope, startBlockID, endBlockID, keyword string, contextBefore, contextAfter, maxDepth int) error {
	switch strings.TrimSpace(detail) {
	case "", "simple", "with-ids", "full":
	default:
		return cliValidationError("--detail must be simple, with-ids, or full")
	}
	if (detail == "with-ids" || detail == "full") && strings.TrimSpace(format) != "xml" {
		return cliValidationError("--detail %s is only supported with --format xml", detail)
	}
	if contextBefore < 0 || contextAfter < 0 {
		return cliValidationError("--context-before and --context-after must be >= 0")
	}
	if maxDepth < -1 {
		return cliValidationError("--max-depth must be >= -1")
	}
	switch strings.TrimSpace(scope) {
	case "", "full", "outline":
	case "range":
		if strings.TrimSpace(startBlockID) == "" && strings.TrimSpace(endBlockID) == "" {
			return cliValidationError("range scope requires --start-block-id or --end-block-id")
		}
	case "keyword":
		if strings.TrimSpace(keyword) == "" {
			return cliValidationError("keyword scope requires --keyword")
		}
	case "section":
		if strings.TrimSpace(startBlockID) == "" {
			return cliValidationError("section scope requires --start-block-id")
		}
	default:
		return cliValidationError("--scope must be full, outline, range, keyword, or section")
	}
	return nil
}

func validateUpdateFlags(command, format, pattern, blockID string, maxReplacements int, yes bool) error {
	switch command {
	case "append", "overwrite", "str_replace", "block_insert_after":
	case "":
		return cliValidationError("--command is required")
	default:
		return cliValidationError("--command must be append, overwrite, str_replace, or block_insert_after")
	}
	if err := validateDocFormat(format, false); err != nil {
		return err
	}
	if command == "overwrite" && !yes {
		return cliValidationError("--command overwrite requires --yes")
	}
	if command == "str_replace" {
		if strings.TrimSpace(pattern) == "" {
			return cliValidationError("--command str_replace requires --pattern")
		}
		if maxReplacements <= 0 {
			return cliValidationError("--max-replacements must be > 0")
		}
	}
	if command == "block_insert_after" && strings.TrimSpace(blockID) == "" {
		return cliValidationError("--command block_insert_after requires --block-id")
	}
	return nil
}

func validateWikiCreateFlags(spaceID, parentNodeToken, nodeType, objType, originNodeToken string) error {
	if strings.TrimSpace(spaceID) == "my_library" {
		return cliValidationError("tenant-token bot identity does not support --space-id my_library")
	}
	if strings.TrimSpace(spaceID) == "" && strings.TrimSpace(parentNodeToken) == "" {
		return cliValidationError("--space-id or --parent-node-token is required")
	}
	switch nodeType {
	case "origin", "shortcut":
	default:
		return cliValidationError("--node-type must be origin or shortcut")
	}
	switch objType {
	case "docx", "sheet", "mindnote", "bitable", "slides":
	default:
		return cliValidationError("--obj-type must be docx, sheet, mindnote, bitable, or slides")
	}
	if nodeType == "shortcut" && strings.TrimSpace(originNodeToken) == "" {
		return cliValidationError("--origin-node-token is required when --node-type=shortcut")
	}
	if nodeType != "shortcut" && strings.TrimSpace(originNodeToken) != "" {
		return cliValidationError("--origin-node-token can only be used when --node-type=shortcut")
	}
	return nil
}

func validateDriveGrantFlags(token, resourceType, openID, perm string) error {
	if strings.TrimSpace(token) == "" {
		return cliValidationError("--token is required")
	}
	if strings.TrimSpace(openID) == "" {
		return cliValidationError("--open-id is required")
	}
	if strings.TrimSpace(perm) == "" {
		return cliValidationError("--perm is required")
	}
	switch strings.TrimSpace(resourceType) {
	case "docx", "doc", "wiki", "sheet", "bitable", "base", "file", "folder", "mindnote", "slides":
		return nil
	default:
		return cliValidationError("--type must be docx, doc, wiki, sheet, bitable, file, folder, mindnote, or slides")
	}
}

func nestedString(data map[string]any, keys ...string) string {
	var current any = data
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = m[key]
	}
	v, _ := current.(string)
	return strings.TrimSpace(v)
}

func writeFeishuDocSuccess(quiet bool, operation string, target *feishuDocTarget, platform string, res *clouddoc.Response) {
	if quiet {
		return
	}
	writeJSON(os.Stdout, buildFeishuDocSuccessPayload(operation, target, platform, res))
}

func buildFeishuDocSuccessPayload(operation string, target *feishuDocTarget, platform string, res *clouddoc.Response) map[string]any {
	data := map[string]any{}
	if res != nil && res.Data != nil {
		data = res.Data
	}
	out := map[string]any{
		"ok":        true,
		"operation": operation,
		"platform":  platform,
		"data":      data,
	}
	if target != nil {
		out["project"] = target.ProjectName
		out["platform_index"] = target.PlatformPos + 1
	}
	if res != nil {
		if res.LogID != "" {
			out["log_id"] = res.LogID
		}
	}
	promoteStringField(out, "document_id", data, [][]string{{"document_id"}, {"document", "document_id"}})
	promoteStringField(out, "wiki_node_token", data, [][]string{{"wiki_node_token"}, {"node_token"}, {"node", "node_token"}})
	promoteStringField(out, "obj_token", data, [][]string{{"obj_token"}, {"node", "obj_token"}})
	promoteStringField(out, "obj_type", data, [][]string{{"obj_type"}, {"node", "obj_type"}})
	promoteStringField(out, "url", data, [][]string{{"url"}, {"document", "url"}, {"node", "url"}})
	if grants, ok := data["permission_grants"]; ok {
		out["permission_grants"] = grants
	}
	return out
}

func feishuDocExit(operation string, err error, quiet bool) {
	_ = quiet
	writeJSON(os.Stderr, buildFeishuDocErrorPayload(operation, err))
	os.Exit(1)
}

func buildFeishuDocErrorPayload(operation string, err error) map[string]any {
	message := err.Error()
	out := map[string]any{
		"ok":        false,
		"operation": operation,
		"message":   message,
		"error": map[string]any{
			"type":    "internal_error",
			"message": message,
		},
	}
	var cliErr *feishuDocCLIError
	if errors.As(err, &cliErr) {
		out["error"].(map[string]any)["type"] = cliErr.Type
		out["error"].(map[string]any)["message"] = cliErr.Message
		out["type"] = cliErr.Type
		out["message"] = cliErr.Message
	} else {
		var apiErr *clouddoc.APIError
		if errors.As(err, &apiErr) {
			e := out["error"].(map[string]any)
			e["type"] = apiErr.Type
			e["message"] = apiErr.Message
			out["type"] = apiErr.Type
			out["message"] = apiErr.Message
			if apiErr.APIPath != "" {
				e["api_path"] = apiErr.APIPath
				out["api_path"] = apiErr.APIPath
			}
			if apiErr.Code != 0 {
				e["code"] = apiErr.Code
				out["code"] = apiErr.Code
			}
			if apiErr.HTTPStatus != 0 {
				e["http_status"] = apiErr.HTTPStatus
				out["http_status"] = apiErr.HTTPStatus
			}
			if apiErr.LogID != "" {
				e["log_id"] = apiErr.LogID
				out["log_id"] = apiErr.LogID
			}
		}
	}
	return out
}

func promoteStringField(out map[string]any, outKey string, data map[string]any, paths [][]string) {
	for _, path := range paths {
		if v := nestedAnyString(data, path...); v != "" {
			out[outKey] = v
			return
		}
	}
}

func nestedAnyString(data map[string]any, keys ...string) string {
	var current any = data
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = m[key]
	}
	v, _ := current.(string)
	return strings.TrimSpace(v)
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func cliValidationError(format string, args ...any) error {
	return &feishuDocCLIError{Type: "validation_error", Message: fmt.Sprintf(format, args...)}
}

func cliConfigError(format string, args ...any) error {
	return &feishuDocCLIError{Type: "config_error", Message: fmt.Sprintf(format, args...)}
}

func cliAPIShapeError(message, logID string) error {
	msg := message
	if strings.TrimSpace(logID) != "" {
		msg += " (log_id=" + strings.TrimSpace(logID) + ")"
	}
	return &feishuDocCLIError{Type: "api_error", Message: msg}
}

func printFeishuDocUsage() {
	fmt.Println(`Usage: cc-connect feishu doc <command> [options]

Commands:
  create   Create a cloud document from content
  fetch    Fetch document content, defaulting to XML with block IDs
  update   Update document content with append, overwrite, str_replace, or block_insert_after`)
}

func printFeishuWikiUsage() {
	fmt.Println(`Usage: cc-connect feishu wiki <command> [options]

Commands:
  create    Create a wiki node
  resolve   Resolve a wiki URL/token to its backing node metadata`)
}

func printFeishuDriveUsage() {
	fmt.Println(`Usage: cc-connect feishu drive <command> [options]

Commands:
  grant   Grant a user open_id access to a Drive resource`)
}
