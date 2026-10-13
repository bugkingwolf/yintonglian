# GORM改造说明

数据访问层由database/sql手写CRUD改为GORM v1.31.1，SQLite方言v1.6.0。GORM通过Conn和DriverName连接modernc.org/sqlite，保留免CGO构建。go.mod中存在方言包引入的mattn/go-sqlite3间接依赖，但服务使用modernc连接。

## 改动范围

- Store.DB为*gorm.DB；作品、分类、客户、设置用GORM查询与OnConflict更新。
- 管理员、会话、上传记录使用结构体模型以及Create、Take、Update、Delete。
- 种子初始化、唯一主推荐、会话发放、密码修改保留显式事务；事务通过Error检查失败并回滚。
- 无记录统一使用gorm.ErrRecordNotFound；外键冲突仍使用现有HTTP错误映射。
- GORM SQL日志设为Silent，避免认证哈希、Token摘要和内容出现在SQL日志中；服务错误仍由HTTP层处理。

## 数据兼容与边界

保留原来的七张表与JSON payload字段，不自动重建表，不重置账号、不改会话格式、不迁移上传文件。客户端仍接收原JSON结构。数据库初始化保留显式DDL维护CHECK和外键约束；备份的VACUUM INTO仍为参数化原生SQL。

本轮是ORM改造，不是数据库规范化。后续如需更多数据库条件筛选或报表，可新增迁移版本逐步拆分内容字段；不把AutoMigrate视为完整的数据迁移方案。仍适用于单实例低频写入的展示后台。

## 验证

12项Go测试，包括用原生SQL创建的旧库兼容测试；验证旧内容读取、原始payload不变、更新、外键保护和空客户SQL NULL。其余HTTP测试覆盖认证、上传、事务、备份恢复等。

go vet、race检测、CGO_ENABLED=0构建与4项前端逻辑测试通过。存量服务切换前备份，切换后比对七张表的数据摘要。

GORM引入的x/text已升级到v0.39.0，修复扫描发现的GO-2026-5970；项目最低Go版本由1.24升到1.25。
