# 内容接口

官网和小程序共享以下只读接口，不需要用户登录。

| 方法与路径 | 作用 |
| --- | --- |
| GET /api/public/config | 分类、客户、公司内容和启用的平台入口 |
| GET /api/public/case-list | 已发布案例摘要分页，支持category、client、type、q、featured=1过滤 |
| GET /api/public/cases/:id | 已发布案例详情，草稿或下架返回404 |
| GET /media/:filename | 附件；公开内容引用时可匿名读取，其他情况需要管理员会话 |

后台接口使用HttpOnly会话Cookie。所有写请求需要同源Origin与X-CSRF-Token；首次本机设置和登录验证同源，后续获取session返回CSRF令牌。

| 接口 | 作用 |
| --- | --- |
| GET /api/admin/session | 登录状态、首次设置状态、CSRF |
| POST /api/admin/setup | 本机首次初始化，生产禁用 |
| POST /api/admin/login、logout、password | 登录、退出、修改密码 |
| GET /api/admin/config | 后台配置及案例统计，不含案例与素材列表 |
| POST /api/admin/cases、PUT或DELETE /api/admin/cases/:id | 案例维护 |
| POST /api/admin/categories或clients、PUT或DELETE /:id | 分类和客户维护 |
| PUT /api/admin/company | 公司和首页配置 |
| PUT /api/admin/platforms | GEO与漫剧两项平台配置 |
| POST /api/admin/uploads | 原始二进制流上传，X-File-Name携带编码后的原文件名 |
| DELETE /api/admin/uploads/:id | 删除没有引用的附件 |
| GET /api/admin/export | 导出内容，排除认证信息 |

错误统一返回JSON `{ "error": "说明" }`。令牌不会作为URL参数传输。公开接口不允许客户端绕过发布状态筛选。

公司配置新增 `teamMembers` 数组（管理团队 `management`、执行团队 `execution`）。GET /api/public/config仅返回enabled成员并按order排序；管理员内容接口保留全部。PUT /api/admin/company支持整组更新，姓名、职位必填，最多50人；photo为系统图片地址，id空时自动生成。其他公司字段未提交时保持不变。


## 分页查询

各端统一使用以下按需查询接口。旧的全量接口 `GET /api/public/cases` 和 `GET /api/admin/content` 已删除，请使用摘要分页、单条详情和配置接口。

| 接口 | 用途 |
| --- | --- |
| GET /api/public/home | 首页主推荐、精选、每业务1条封面摘要、已发布案例客户ID。精选最多12条 |
| GET /api/public/case-list | 公开案例摘要分页，固定只查已发布 |
| GET /api/admin/config | 后台公司、分类、客户、平台、全局案例统计及分类客户关联计数，不含案例与素材记录 |
| GET /api/admin/cases | 管理员案例摘要分页 |
| GET /api/admin/cases/:id | 管理员单条完整详情，编辑、复制、发布下架前使用 |
| GET /api/admin/uploads | 上传素材分页，支持kind=image或video、q文件名搜索 |

分页响应：`{items,total,page,pageSize}`。page从1起，超出末页时返回实际末页；pageSize最大100。公开案例默认12条，后台页面请求20条，素材默认24条，素材选择器请求12条。按展示排序加ID稳定排序，素材按上传时间降序加ID降序。筛选与LIMIT/OFFSET在SQLite执行，不在Go中先读全表再截取。

案例筛选支持category、client、type、q、featured=1、hero=1；后台另支持status。q最多120字符，搜索标题、摘要、正文及客户名，输入中的百分号和下划线视为普通字符。relatedTo指定已发布案例ID，返回同客户或同业务的其他已发布摘要，可配pageSize=2。

案例摘要含id、title、categoryId、clientId、type、summary、cover、hasVideo、status、featured、hero、demo、order、updatedAt，不返回description、gallery、video、experienceUrl、services。公开详情仍用 `/api/public/cases/:id`，保持既有分享路径和发布状态校验。

官网配置短缓存2分钟，切页按需加载相关数据；小程序配置同样短缓存2分钟，下拉刷新绕过缓存。新小程序首页用home，案例列表分页追加，详情独立读取；关于我们、平台页和名片不再请求全部案例。必须先部署新服务端，再上传小程序新版。旧开发版依赖的全量接口已删除，服务端升级后必须更新小程序代码；不可继续使用旧开发版。
