# 银通联展示系统 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现自有服务器可部署的官网、共用内容后台和微信小程序源码。

**Architecture:** 单体内容服务，SQLite持久化，公开接口仅返回已发布案例；官网与微信原生小程序共用接口。管理员身份、上传文件及原平台登录相互独立。

**Tech Stack:** Node.js 24、node:sqlite、原生HTML/CSS/JavaScript、微信原生小程序；不依赖外部前端构建服务。

## 决策记录

- ADR 01：在现有本机与后续自有服务器运行，使用Node单体而非托管云函数，减少迁移与账号依赖。以单管理员、少量内容更新为目标，SQLite足够；后续大量并发写入再考虑迁移数据库。
- ADR 02：原生前端避免依赖安装与构建；CSS令牌和共享API保持一致。官网和小程序分别适配，不假定网页可以直接成为原生小程序。
- ADR 03：布局固定、内容可编辑，不制作拖拽建站。业务分类与作品类型分开，案例可关联客户和多种媒体。
- ADR 04：首次本机访问设置管理员密码，生产部署通过命令行初始化；不使用通用默认密码。密码scrypt哈希，HttpOnly会话、CSRF验证和登录限流。
- ADR 05：上传文件不放公开静态目录。仅已发布案例引用的文件公开可读；草稿或下架附件需管理员身份。平台外链仅允许HTTP/HTTPS。

## Task 1 内容服务

Files：server/app.mjs、server/store.mjs、server/seed.mjs、server/security.mjs、test/api.test.mjs。

1. 建立分类、客户、案例、配置、附件、管理员和会话表。
2. 实现公开案例过滤与详情、后台登录与内容CRUD、二进制上传及范围请求。
3. 运行真实HTTP测试：发布状态、会话与CSRF、文件魔数、草稿文件访问、分类删除保护、数据重启持久化。

## Task 2 官网与后台

Files：public/index.html、public/site.css、public/site.js、public/admin/index.html、public/admin/admin.css、public/admin/admin.js。

1. 从PPT页面提取示例缩略图，避免生成图充当实际项目素材。
2. 实现白金官网、业务与案例筛选、客户集合、图文与视频详情、平台入口、关于我们、分享与联系。
3. 实现首次设置、登录、案例编辑、上传进度、分类客户维护、公司内容与平台链接维护。
4. 检查PC与手机视口、空状态、输入错误、导航与保存后的即时展示。

## Task 3 小程序

Files：miniprogram/app.js、app.json、app.wxss、utils/api.js、pages/*、project.config.json。

1. 完成首页、业务与案例、案例详情、平台入口和关于页面。
2. 共用公开API，使用已发布内容；实现预览图、播放视频、分享路径。
3. 外链使用复制并提示电脑端使用，未经域名配置和移动适配验证不声称能嵌入。
4. 进行语法和配置检查，标注微信开发者工具与真机验证需企业AppID及HTTPS域名。

## Task 4 交付

Files：README.md、docs/API.md、docs/DEPLOYMENT.md、docs/VERIFICATION.md、deploy/*。

1. 本机预览与真实API测试。
2. 提供运行、初始化管理员、备份恢复与部署说明。
3. 打包不含密码、会话、数据库及用户上传的源代码。
4. 展示本机可用版本，并如实标注外部平台和小程序上线的待验证项。
