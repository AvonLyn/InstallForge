# InstallForge

InstallForge 是一个离线可用的自动化脚本生成器 MVP。后端使用 Go 标准库提供本地 HTTP 服务，前端页面通过 `embed` 打包在二进制中。用户通过 API 保存安装配方（recipe），即可预览生成的 `install.sh`，并导出包含脚本与资产的离线 Bundle。

## 功能概览（以当前代码为准）

- 本地 HTTP 服务（默认 `127.0.0.1:8080`）
- 项目与资产存储在本地目录 `data/projects/<id>`
- Recipe 校验（缺失字段/模式错误给出错误或警告）
- 预览生成：`install.sh`、`README.txt`、`recipe.json`（pretty）
- 导出 Bundle：`install.sh` + `recipe.json` + `README.txt` + `assets/`
- 内置前端：`webembed/web/index.html`（当前为极简页面）

## 快速开始

前置要求：Go 1.21。

```bash
# 运行本地服务

go run ./cmd/asg

# 可指定端口
PORT=9090 go run ./cmd/asg
```

打开浏览器访问：`http://127.0.0.1:8080`。

项目数据默认写入 `data/projects/`。

## Recipe 数据结构

最小结构如下（参考 `internal/recipe/recipe.go`）：

```json
{
  "schema_version": "1.0",
  "project": {
    "id": "uuid",
    "name": "demo",
    "description": "",
    "target": ["oracle_linux_6_9", "kylinsec_3_4"]
  },
  "vars": {
    "INSTALL_ROOT": "/opt/demo",
    "LOG_DIR": "/var/log/asg"
  },
  "steps": [
    {
      "id": "step-1",
      "name": "mkdir",
      "type": "mkdir",
      "config": {"path": "/opt/demo"}
    }
  ]
}
```

## 支持的 Step 类型

渲染与校验覆盖以下类型（见 `internal/recipe/validate.go` 与 `internal/render/render.go`）：

- 文件/目录：`mkdir`、`copy`、`chmod`、`chown`
- 解压/安装：`extract_tar_gz`、`extract_zip`、`rpm_install`
- 配置编辑：`append_lines`、`delete_lines`、`replace`
- 命令与服务：`run_cmd`、`service_sysv`、`service_systemd`、`auto_service`

## API 概览

服务端接口位于 `internal/api/handlers.go`：

- `GET /api/projects`：列出项目
- `POST /api/projects`：创建项目
- `GET /api/projects/{id}`：读取 recipe
- `PUT /api/projects/{id}`：保存 recipe（返回校验问题）
- `GET /api/projects/{id}/assets`：列出资产
- `POST /api/projects/{id}/assets`：上传资产（multipart）
- `POST /api/projects/{id}/generate`：生成预览
- `POST /api/projects/{id}/export`：导出 bundle（返回本地路径）

示例：

```bash
# 创建项目
curl -X POST http://127.0.0.1:8080/api/projects \
  -H 'Content-Type: application/json' \
  -d '{"name":"demo","description":"","target":["oracle_linux_6_9"]}'

# 保存 recipe
curl -X PUT http://127.0.0.1:8080/api/projects/<id> \
  -H 'Content-Type: application/json' \
  -d @recipe.json

# 预览生成
curl -X POST http://127.0.0.1:8080/api/projects/<id>/generate

# 导出 bundle（返回 path）
curl -X POST http://127.0.0.1:8080/api/projects/<id>/export \
  -H 'Content-Type: application/json' \
  -d '{"format":"dir"}'
```

## 生成脚本说明

`internal/render/render.go` 使用模板生成 `install.sh`，具备：

- root 检查（非 root 直接退出）
- 日志输出到 `{{LOG_DIR}}/install-YYYYMMDD-HHMMSS.log`
- preflight 命令检测（根据 steps 推导）
- 按步骤输出进度与日志

## 目录结构

```
cmd/asg/main.go        # 入口，启动 HTTP 服务
internal/api/          # API 路由与处理逻辑
internal/recipe/       # Recipe 数据结构与校验
internal/render/       # install.sh/README 生成
internal/store/        # 本地文件存储（recipe/asset）
webembed/embed.go      # 前端资源 embed
webembed/web/index.html# 前端页面（极简）
需求文档.txt            # 详细需求与规划文档
```

## 需求文档

`需求文档.txt` 是项目的完整需求与 MVP 规划，涵盖 UI 规格、Step Schema、校验规则与导出结构等。

## License

MIT License，详见 `LICENSE`。
