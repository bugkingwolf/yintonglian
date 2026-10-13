# 内容接口

官网和小程序共享以下只读接口，不需要用户登录。

| 方法与路径 | 作用 |
| --- | --- |
| GET /api/public/config | 分类、客户、公司内容和启用的平台入口 |
| GET /api/public/cases | 已发布案例，支持category、client、type、q、featured=1过滤 |
| GET /api/public/cases/:id | 已发布案例详情，草稿或下架返回404 |
| GET /media/:filename | 附件；公开内容引用时可匿名读取，其他情况需要管理员会话 |

后台接口使用HttpOnly会话Cookie。所有写请求需要同源Origin与X-CSRF-Token；首次本机设置和登录验证同源，后续获取session返回CSRF令牌。

| 接口 | 作用 |
| --- | --- |
| GET /api/admin/session | 登录状态、首次设置状态、CSRF |
| POST /api/admin/setup | 本机首次初始化，生产禁用 |
| POST /api/admin/login、logout、password | 登录、退出、修改密码 |
| GET /api/admin/content | 后台全部案例与配置、素材记录 |
| POST /api/admin/cases、PUT或DELETE /api/admin/cases/:id | 案例维护 |
| POST /api/admin/categories或clients、PUT或DELETE /:id | 分类和客户维护 |
| PUT /api/admin/company | 公司和首页配置 |
| PUT /api/admin/platforms | GEO与漫剧两项平台配置 |
| POST /api/admin/uploads | 原始二进制流上传，X-File-Name携带编码后的原文件名 |
| DELETE /api/admin/uploads/:id | 删除没有引用的附件 |
| GET /api/admin/export | 导出内容，排除认证信息 |

错误统一返回JSON `{ "error": "说明" }`。令牌不会作为URL参数传输。公开接口不允许客户端绕过发布状态筛选。

公司配置新增 `teamMembers` 数组（管理团队 `management`、执行团队 `execution`）。GET /api/public/config仅返回enabled成员并按order排序；管理员内容接口保留全部。PUT /api/admin/company支持整组更新，姓名、职位必填，最多50人；photo为系统图片地址，id空时自动生成。其他公司字段未提交时保持不变。
