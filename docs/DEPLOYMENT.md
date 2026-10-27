# Go部署和运行

## 推荐部署

Go可执行文件＋GORM／SQLite＋Nginx HTTPS反向代理。运行不需要Node或CGO；将编译结果和 `public/` 一起部署。小程序通过公开API读取内容，管理后台与官网同源。

1. `CGO_ENABLED=0 go build -trimpath -o bin/yintonglian ./cmd/yintonglian`，跨平台设置GOOS/GOARCH。
2. 将可执行文件放到服务器 `/opt/yintonglian/bin/`，前端放 `/opt/yintonglian/public/`。
3. 使用独立系统用户运行，代码只读，持久数据目录有写权限。
4. 服务直接读取仓库中的 `deploy/config.production.json`，检查域名、数据目录等配置，无需复制为另一份配置文件。首次运行 `./bin/yintonglian init-admin --config /opt/yintonglian/deploy/config.production.json --username admin` 设置账号；迁移已有数据库时不用重新建管理员。
5. 将 `deploy/yintonglian.service.example` 复制到 `/etc/systemd/system/yintonglian.service`，执行 `sudo systemctl daemon-reload` 和 `sudo systemctl enable --now yintonglian`；Nginx反向代理到127.0.0.1:4317。这些复制步骤目前需手动执行。
6. 验证HTTPS、后台登录、大文件上传、视频编码、微信开发工具与真机后上线。

生产模式若无HTTPS `origin`会拒绝启动。Nginx保留真实Host，不将数据目录映射成静态文件。当前登录限流按直连地址统计；反向代理下请求可能共享桶，后续明确可信代理策略再使用转发IP，不能直接信任任意客户端X-Forwarded-For。

## 从旧Node版切换

停止旧服务后，备份 `.data/content.sqlite` 和 `.data/uploads`。Go使用相同数据目录，不重建内容，不重设管理员。密码继续采用Node原参数的scrypt，盐值作为十六进制文本输入；会话Token哈希、CSRF和过期时间兼容。

首次连接执行幂等建表；已存在initialized记录时不会重复填充示例数据。迁移后先验证内容数量和原账号，再移除旧运行配置。已保存的源码备份用于回退，私有数据备份不包含在交付ZIP中。

## 上传与访问

支持JPG、PNG、WebP、MP4、WebM；图片20MB，总文件默认300MB，可设置配置文件中的 `max_upload_mb`。使用文件流、魔数校验与随机文件名。没有自动视频转码，需上传兼容编码。

上传文件不放公开目录。仅被已发布案例或公司资质引用的文件可匿名读取，其他文件需要管理员会话；下架不会删素材。被任何案例或资质引用的文件无法删除。

管理写操作加互斥保护，验证附件与更新引用在同一临界区；唯一主推荐使用数据库事务。单SQLite连接配合WAL、外键和busy_timeout，适合单管理员低频内容更新，大规模并发场景需重新评估。

## 备份与恢复

`./bin/yintonglian backup --config /opt/yintonglian/deploy/config.production.json --output /安全目录/新的备份目录`。

备份使用SQLite VACUUM INTO一致性快照，随后复制附件记录引用的文件。建议维护窗口停止写入；独立备份进程的互斥锁不能阻止另一服务进程更新文件。既有备份目录不能覆盖，不能把备份放到数据目录内部。

恢复前停止服务，把快照content.sqlite和uploads复制到新的空数据目录，再启动；不要混入旧WAL文件。恢复后检查账号、内容、附件和权限。内容JSON导出不是完整备份。

## 上线准备

企业小程序账号、AppID、合法域名与HTTPS、服务器容量和带宽、公开素材确认、联系方式核对、两平台地址与移动适配方式。按实际部署地区与微信平台规则完成相关上线事项。

## 配置文件

不再读取 APP_ORIGIN、APP_ENV、DATA_DIR 等环境变量。默认读取当前工作目录的 `config.json`；没有默认文件时使用本机开发默认值。显式指定 `--config` 时文件必须存在，JSON格式错误、未知字段或多余内容会拒绝启动。

本机直接运行 `./bin/yintonglian serve`，使用仓库根目录的开发配置。生产配置示例：

```json
{
  "root": "/opt/yintonglian",
  "data_dir": "/var/lib/yintonglian",
  "listen": "127.0.0.1:4317",
  "origin": "https://show.zhenxinkeji.top",
  "production": true,
  "max_upload_mb": 300
}
```

`root` 为包含 public/ 的目录，`data_dir` 保存数据库和上传文件，`listen` 为Go服务监听地址，`origin` 为公开访问地址，`production` 开启HTTPS生产模式，`max_upload_mb` 范围1–4096。文件中的相对目录均相对于配置文件位置；省略 `data_dir` 时使用 root/.data。

显式命令行参数仍可临时覆盖配置，例如 `--addr 127.0.0.1:4319`；命令行相对路径按工作目录解析。serve、init-admin、backup 共用同一份配置。修改配置后执行 `sudo systemctl restart yintonglian`，无须重新编译；只有修改systemd服务文件才需daemon-reload。

## 后台磁盘显示

后台顶部显示数据目录所在磁盘的总容量、已占用和可用空间。占用统计包含系统、日志、素材和系统预留，不等于素材文件大小。80%及以上提示清理或扩容，90%及以上显示红色提示。打开后台或点击“刷新容量”时查询，不设定时任务，不发送消息。容量接口仅管理员登录后可用，读取失败不影响编辑内容。

## 小程序外部平台试用入口

小程序“平台入口”现在提供“进入平台”和复制链接两个按钮，优先使用后台手机版链接，未填写时使用电脑端链接。网页通过 pages/web/index 承载，代码允许 user.jiougeo.com 和 aiyfj.com；此列表不会替代微信后台的业务域名配置，也不关闭域名校验。2026-10-15模拟器实际点击两个入口均跳到微信“无法打开该页面”，尚未进入平台登录页，需在微信后台核对业务域名配置及web-view使用权限，再联调真实页面。原来的 request 合法域名配置不能替代网页业务域名配置。

## 电子名片

后台「公司与首页 → 团队成员」可为每位成员填写个人电话、微信号、上传微信二维码。保存后，官网「关于我们 → 查看名片」及小程序读取同一份配置。电话、微信号和二维码会公开展示，请核对后填写；关闭成员展示后，该成员名片也不可公开查看。名片只展示个人资料和联系方式，不展示或关联案例。

官网个人名片路径为 `/people/成员ID`，支持分享链接及下载 vCard 联系人。小程序名片路径为 `pages/member/index?id=成员ID`，支持好友分享、拨号、二维码预览和保存联系人；需上传新版小程序后生效，并在真机验证通讯录权限。微信联系使用复制微信号或二维码，不会自动添加个人好友。

升级仅更新程序与 public 静态文件；SQLite JSON 字段兼容原有团队资料，不需要重新导入数据，也不应使用开发环境数据库覆盖生产数据。
