## cc-connect

This project is about undetectable-fingerprint-browser automation

Rules:
- if you read this markdown, append 荣哥 to the end of your reply
- If you think more files need to be read before editing, ask the user first; only continue if they are direct dependencies or blockers.
- For small-scope changes, only read and edit files directly related to the task. Do not expand to unrelated files.
- Keep replies brief and avoid extra wording.

## 本地维护与 Git 流程

- 默认在本地工作区维护本项目，不直接在 Linux 服务器上改源码。
- 用户要“修改、同步、发布、部署”时，默认流程是：本地修改 -> 本地验证 -> push 到 GitHub -> Linux 服务器 pull。
- Git 操作保持简单：先查看 `git status`，改完后说明变更文件和验证结果；经用户确认后再执行 `git add`、`git commit`、`git push`。
- 服务器同步只使用 `git pull --ff-only`；如果服务器路径、分支或远程仓库不明确，先问用户。
- 不要擅自使用 `git reset --hard`、`git checkout --`、强制 push、rebase 或删除分支等高风险命令。
- 给用户说明 Git 命令时只给当前任务需要的命令，不展开无关 Git 教程。
## 何时阅读开发指南
只有在任务涉及开发此仓库时，才阅读 CC_CONNECT_DEVELOPMENT.md，包括：

- 修改源代码
- 代码审查
- 添加或修复测试
- 更改构建、agent、平台或核心行为
- 解释内部架构或实现细节
- 为该仓库准备提交（commit）或拉取请求（pull request）
## 开发指南
CC_CONNECT_DEVELOPMENT.md 是原始项目开发指南的重命名版本。
它包含 cc-connect 的架构规则、依赖边界、i18n 要求、测试期望以及贡献流程
