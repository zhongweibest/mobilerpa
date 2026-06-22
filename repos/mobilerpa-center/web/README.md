# web

这是中心仓库中的后台前端应用目录。

## 推荐技术栈
- Vue 3
- Vite
- TypeScript
- Element Plus
- Pinia
- Vue Router

## 选择原因
- 当前阶段前后端需要联动发布
- 标准 Vue Admin 应用已经足够满足后台需求
- 不需要引入 Go 生态专用的后台框架

## 文档约定
本目录下的设计说明和开发文档默认使用中文。

## 本地默认配置

`vite` 开发模式会自动读取 `web/.env.development`。
开发环境默认通过 Vite 代理把 `/api` 和 `/ws` 转发到 `VITE_API_PROXY_TARGET`，所以浏览器不会直接跨域访问中心服务。
如果需要修改本地联调目标地址，直接编辑该文件里的 `VITE_API_PROXY_TARGET` 即可，不需要手动设置 PowerShell 环境变量。

生产构建会读取 `web/.env.production`。发布前请把其中的 `VITE_API_BASE_URL` 改成真实线上中心服务地址，再执行构建。
