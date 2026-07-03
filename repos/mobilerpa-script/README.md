# 脚本仓库说明

该仓库用于维护可被 MobileRPA 平台上传、下发、调试和执行的脚本源码。

当前仓库已经统一为一套“TypeScript 开发 + JavaScript 发布”的工程模式：

- `packages/`：脚本源码工程目录，使用 Node + TypeScript 开发。
- `publish/脚本名/版本号/`：发布产物目录，继续兼容中心服务现有上传和下发方式。

手机端和 AutoJs6 端始终只运行构建后的 JavaScript，不直接运行 TypeScript 源码。

## 当前目录结构

- `packages/shared`
  统一公共 helper、运行时类型、取消检查、应用启动等共享能力。
- `packages/<script-package>`
  每个脚本一个独立包，例如 `packages/open-qq`、`packages/shoppe-sync`。
- `publish/<script_name>/<version>/`
  最终发布目录，例如 `publish/open_qq/v0.1.2/`、`publish/shoppe_sync/v0.1.2/`。
- `docs/`
  脚本仓库自身说明与接入清单。
- `tools/build-script-package.js`
  通用单脚本发布器。
- `tools/release-all.js`
  全量发布入口。

## 发布产物约定

每个脚本版本目录下统一生成以下文件：

- `index.js`
  真正给 Agent 执行的脚本入口。
- `index_debug.js`
  给 AutoJs6 本地直跑调试使用的入口。
- `manifest.json`
  发布清单，描述当前版本包含哪些文件。
- `vX.Y.Z.zip`
  对应版本的压缩包，可直接用于上传或归档。

## 推荐开发流程

1. 在 `packages/` 下维护脚本源码。
2. 公共能力优先放进 `packages/shared`，不要在各脚本里重复复制。
3. 本地先执行类型检查。
4. 构建并回写 `publish/` 发布目录。
5. 使用 `index_debug.js` 做真机调试。
6. 调试通过后，再由中心服务上传和下发。

## 常用命令

在仓库根目录执行：

```bash
cd D:\dev\code\mobilerpa\repos\mobilerpa-script
npm install
npm run check
npm run release
```

命令说明：

- `npm run check`
  对所有 `packages/*` 执行 TypeScript 类型检查。
- `npm run build`
  按 workspace 执行各包构建脚本。
- `npm run release`
  按统一发布流程逐个构建脚本，并自动生成：
  `index.js`、`index_debug.js`、`manifest.json`、`zip`

## 新增或升级脚本版本

建议统一按以下步骤操作：

1. 在对应 `packages/<script-package>/script.config.json` 中维护版本号。
2. 在源码中同步更新 `SCRIPT_VERSION`。
3. 执行 `npm run release`。
4. 确认 `publish/<script_name>/<version>/` 目录下产物齐全。
5. 用 `index_debug.js` 做本地调试。
6. 再通过中心服务上传和下发。

## 当前已接入工程化的脚本

- `open_qq`
- `open_weichat`
- `open_douyin`
- `open_toutiao`
- `open_xiaohongshu`
- `shoppe_sync`

这些脚本都已经具备：

- TS 源码包
- 统一构建入口
- 统一调试入口
- 统一 `manifest.json`
- 统一版本压缩包输出

## 验收建议

每次版本升级后，建议至少核对以下内容：

1. `npm run check` 通过。
2. `npm run release` 通过。
3. 发布目录存在 `index.js`、`index_debug.js`、`manifest.json`、`zip`。
4. 中心服务上传成功。
5. 手机端下发成功。
6. AutoJs6 本地调试入口可直接运行。
7. 任务执行、停止、结果回传符合预期。

## 统一约定

1. 默认使用中文文档、中文注释、中文提交信息。
2. 新脚本优先放在 `packages/` 下开发，不再直接手改发布目录源码。
3. 发布目录视为构建产物，源代码以 `packages/` 为准。
4. 关键执行步骤应统一上报进度，避免只依赖最终成功或失败。
