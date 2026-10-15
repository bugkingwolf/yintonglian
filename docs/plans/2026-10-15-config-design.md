# 配置文件启动

按用户要求，服务直接读JSON，不依赖环境变量。沿用标准库，无新增依赖。

- 默认读取工作目录config.json，缺省文件保留本机默认值；生产systemd显式--config指定文件，缺失或解析失败拒绝启动。
- 根目录config.json用于开发，deploy/config.production.json由服务器直接读取，无需复制配置文件，域名show.zhenxinkeji.top，独立数据目录/var/lib/yintonglian。
- 相对目录按配置文件所在目录解析；命令行参数保留，显式参数优先。
- serve、init-admin、backup共享配置；配置变更后重启，无热加载。
- 严格校验未知字段、JSON内容、上传上限；配置读取失败不会初始化数据库。
- 验证文件路径、三种命令的一致性、参数覆盖、无效输入和环境变量不再生效，再运行Go回归测试和Linux编译。
