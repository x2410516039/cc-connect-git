package ccconnect

const ConfigExampleTOML = `# Minimal cc-connect config example for this trimmed build.

[[projects]]
name = "default"
work_dir = "."

[projects.agent]
type = "codex"

[[projects.platforms]]
type = "feishu"
app_id = "cli_xxx"
app_secret = "xxx"
`
