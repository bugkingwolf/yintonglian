# Go Migration Implementation Plan

**Goal:** 将Node后端替换为Go，保留现有前端、小程序、接口及SQLite内容。

**Architecture:** Go标准库net/http＋database/sql，modernc.org/sqlite纯Go驱动；scrypt兼容现有Node密码哈希，会话表与Cookie协议保持一致。

**Tech Stack:** Go 1.24+，纯Go SQLite，golang.org/x/crypto。

## 决策

- 按用户技术偏好迁移Go；选择net/http避免引入用户未确认的Web框架。
- 使用纯Go SQLite驱动，CGO_ENABLED=0编译，便于跨平台部署。
- 保持API、表结构、字段、管理员哈希与会话兼容；不重置用户数据或账号。
- 服务端拆分为类型、存储、安全、公开接口、认证、管理、媒体、静态页面，避免所有路由集中在单文件。

## 步骤

1. 导出无凭据的默认种子数据，建立Go模块和分层代码。
2. 实现SQLite兼容读写与事务，覆盖唯一主推荐及引用保护。
3. 迁移认证、上传、公开接口和后台CRUD，保持错误响应协议。
4. 用httptest.Server执行真实HTTP集成测试，验证旧哈希及会话兼容。
5. 编译纯Go二进制，运行vet与race检查；继续执行前端逻辑检查。
6. 备份当前数据库和附件，停止旧Node服务，使用同一数据目录启动Go服务；验证原URL。
7. 删除旧后端源码与运行依赖，更新部署文档、运行命令和交付包。
