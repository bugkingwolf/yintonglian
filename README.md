# 银通联官网与微信小程序

服务端已迁移为 **Go**。官网和管理后台仍为原生HTML/CSS/JavaScript，小程序仍为微信原生JavaScript/WXML/WXSS，两端调用同一套Go接口。

## 技术栈

- Go 1.25+，标准库 `net/http`，不引入Gin等Web框架。
- GORM＋SQLite（底层 `modernc.org/sqlite` 纯Go驱动），可用 `CGO_ENABLED=0` 编译。
- `golang.org/x/crypto/scrypt`，兼容之前Node版管理员密码。
- `golang.org/x/term`仅用于CLI隐藏密码输入。
- SQLite数据、上传目录、Cookie名称、CSRF和API格式保持兼容，已有账号不重置。

## 启动

```sh
go run ./cmd/yintonglian
```

官网：`http://127.0.0.1:4317`；后台：`http://127.0.0.1:4317/admin`。

首次本机使用后台可创建管理员，密码至少12位。已有管理员则直接使用原账号密码登录。生产环境禁用网页初始化，用命令行创建：

```sh
go run ./cmd/yintonglian init-admin --username admin
```

该命令隐藏密码输入，已存在管理员时拒绝覆盖。没有通用默认密码。

## 编译与部署

```sh
make build
./bin/yintonglian serve --root . --data .data --addr 127.0.0.1:4317
```

服务端运行不需要Node.js，也不需要CGO。部署需要可执行文件与 `public/` 前端资源；默认根目录为当前工作目录，可以通过 `--root` 指定。

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/yintonglian-linux-amd64 ./cmd/yintonglian
```

生产使用 `APP_ENV=production`、`APP_ORIGIN=https://真实域名`、HTTPS反向代理与持久数据目录。详见 `docs/DEPLOYMENT.md`。

## 代码结构

```text
cmd/yintonglian/main.go      启动、初始化管理员、备份命令
internal/app/
  models.go                 JSON类型与前端接口字段
  store.go                  GORM存储模型、SQLite连接与种子初始化
  security.go               scrypt、随机令牌、外链与文本验证
  server.go                 路由、中间件和响应协议
  handlers_auth.go          登录、Cookie会话、CSRF、密码修改
  handlers_public.go        已发布内容与公开配置
  handlers_cases.go         案例维护、唯一首页主推荐
  handlers_settings.go      分类、客户、公司、平台、导出
  handlers_media.go         流式上传、文件头验证与访问权限
  static.go                 官网静态资源与范围请求
  backup.go                 数据库快照与附件备份
  server_test.go            真实HTTP集成与并发验证
  seed.json                 来自PPT的默认展示内容
public/                     官网、管理后台与案例图片
miniprogram/                微信小程序源码
```

表结构保留初版的JSON payload存储，便于无损切换。本轮不将存储重构为完整关系模型，不引入ORM和迁移框架；后续新增schema需制定显式版本迁移。

## 已实现的管理能力

案例新增/编辑、封面和视频上传、项目图片、客户与业务分类、排序、草稿/发布/下架、精选和唯一主推荐；公司内容、资质图片与联系方式；GEO/漫剧平台链接；修改密码和内容导出。

公开接口只返回已发布案例。草稿/下架案例的上传附件匿名不可访问。被案例或资质引用的文件不能删除。PPT提取示例属于公开静态资源，部署前可移除不准备公开的示例。

目前没有原视频和真实平台地址：视频案例展示画面，不伪造可播放文件；平台显示咨询入口。

## 验证

```sh
go test ./... -count=1
go vet ./...
go test -race ./... -count=1
```

Go测试通过 `httptest.Server` 发出真实HTTP请求，使用临时SQLite和上传目录，不修改真实账号或内容。包含旧Node scrypt哈希和会话兼容、权限、CSRF、上传、范围请求、并发主推荐、备份恢复与重启保留。

前端检查可选使用Node，不影响Go运行：

```sh
node scripts/check.mjs
node --test test/*.test.mjs
node scripts/make-preview.mjs
```

`preview/index.html` 是可直接打开的只读展示预览，不包含后台写入能力。

## 小程序

将 `miniprogram/` 导入微信开发者工具，替换占位AppID，修改 `config.js` 中的API域名并配置微信合法域名。服务接口格式不变，因此Go切换不需要重新写小程序页面。

GEO与漫剧默认复制链接并提示电脑使用，不提供平台原生操作。H5默认复制链接；仅在业务域名已确认且配置 `H5_WEBVIEW_HOSTS` 后才进入web-view。

企业账号、微信编译与真机验证、原平台登录及移动适配仍需相应资料。部署说明与验证记录列出边界。

## 备份

```sh
go run ./cmd/yintonglian backup --output /安全目录/新备份名
```

SQLite快照与上传文件同时保留。备份包括管理员哈希和会话，作为私有文件保管。建议维护窗口操作。内容JSON导出不含媒体文件，不能替代完整备份。

## GORM 数据访问层

业务读写采用GORM的Find、Take、Create、Update、Delete、OnConflict及显式事务。底层连接继续使用modernc驱动，仍支持CGO_ENABLED=0；GORM SQLite方言通过Conn接入已有连接，不使用默认CGO驱动。表结构沿用已有版本，内容JSON payload本轮不拆表；保留显式DDL以维持外键、状态与管理员约束，不对存量数据库直接运行AutoMigrate。SQLite专用VACUUM INTO备份仍使用参数化SQL。

详见 [GORM改造说明](docs/GORM-MIGRATION.md)。

## 文件存储

用户上传由统一FileStorage接口管理，默认仍放在 `.data/uploads`，原数据和 `/media/` 地址保持兼容。上传、读取、删除与备份均经过该接口，已预留鉴权后的短时签名直链交付能力。当前没有接入真实OSS，也无需配置云账号。详见 [存储设计](docs/STORAGE.md)。
