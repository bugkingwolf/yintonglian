# Go部署和运行

## 推荐部署

Go可执行文件＋GORM／SQLite＋Nginx HTTPS反向代理。运行不需要Node或CGO；将编译结果和 `public/` 一起部署。小程序通过公开API读取内容，管理后台与官网同源。

1. `CGO_ENABLED=0 go build -trimpath -o bin/yintonglian ./cmd/yintonglian`，跨平台设置GOOS/GOARCH。
2. 将可执行文件放到服务器 `/opt/yintonglian/bin/`，前端放 `/opt/yintonglian/public/`。
3. 使用独立系统用户运行，代码只读，持久数据目录有写权限。
4. 首次运行 `./bin/yintonglian init-admin --root /opt/yintonglian --data /var/lib/yintonglian --username admin` 设置账号。
5. 设置 `APP_ENV=production`、`APP_ORIGIN=https://真实域名`、`DATA_DIR=/var/lib/yintonglian`，反向代理到127.0.0.1:4317。
6. 验证HTTPS、后台登录、大文件上传、视频编码、微信开发工具与真机后上线。

生产模式若无HTTPS APP_ORIGIN会拒绝启动。Nginx保留真实Host，不将数据目录映射成静态文件。当前登录限流按直连地址统计；反向代理下请求可能共享桶，后续明确可信代理策略再使用转发IP，不能直接信任任意客户端X-Forwarded-For。

## 从旧Node版切换

停止旧服务后，备份 `.data/content.sqlite` 和 `.data/uploads`。Go使用相同数据目录，不重建内容，不重设管理员。密码继续采用Node原参数的scrypt，盐值作为十六进制文本输入；会话Token哈希、CSRF和过期时间兼容。

首次连接执行幂等建表；已存在initialized记录时不会重复填充示例数据。迁移后先验证内容数量和原账号，再移除旧运行配置。已保存的源码备份用于回退，私有数据备份不包含在交付ZIP中。

## 上传与访问

支持JPG、PNG、WebP、MP4、WebM；图片20MB，总文件默认300MB，可设MAX_UPLOAD_MB。使用文件流、魔数校验与随机文件名。没有自动视频转码，需上传兼容编码。

上传文件不放公开目录。仅被已发布案例或公司资质引用的文件可匿名读取，其他文件需要管理员会话；下架不会删素材。被任何案例或资质引用的文件无法删除。

管理写操作加互斥保护，验证附件与更新引用在同一临界区；唯一主推荐使用数据库事务。单SQLite连接配合WAL、外键和busy_timeout，适合单管理员低频内容更新，大规模并发场景需重新评估。

## 备份与恢复

`./bin/yintonglian backup --data /var/lib/yintonglian --output /安全目录/新的备份目录`。

备份使用SQLite VACUUM INTO一致性快照，随后复制附件记录引用的文件。建议维护窗口停止写入；独立备份进程的互斥锁不能阻止另一服务进程更新文件。既有备份目录不能覆盖，不能把备份放到数据目录内部。

恢复前停止服务，把快照content.sqlite和uploads复制到新的空数据目录，再启动；不要混入旧WAL文件。恢复后检查账号、内容、附件和权限。内容JSON导出不是完整备份。

## 上线准备

企业小程序账号、AppID、合法域名与HTTPS、服务器容量和带宽、公开素材确认、联系方式核对、两平台地址与移动适配方式。按实际部署地区与微信平台规则完成相关上线事项。
