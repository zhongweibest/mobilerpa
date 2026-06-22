# 脚本仓库说明

该仓库用于维护可被 MobileRPA 平台上传、下发、调试和执行的脚本源码。

## 目录约定

- 脚本按 `脚本名/版本号/` 组织，例如：`open_qq/v0.1.0/`。
- 每个脚本版本目录下至少必须包含 `index.js`。
- 建议同时保留 `index_debug.js`，用于真机直接调试。

## 脚手架生成

使用以下命令创建一个新的脚本版本脚手架：

```bash
node tools/create-script-scaffold.js --script-name open_demo --version v0.1.0
```

如需覆盖已存在文件：

```bash
node tools/create-script-scaffold.js --script-name open_demo --version v0.1.0 --force
```

## 开发流程建议

1. 使用脚手架创建脚本目录。
2. 在 `index.js` 中补充真实业务逻辑。
3. 在 AutoJs6 中直接运行 `index_debug.js` 做真机调试。
4. 调试通过后，再将目录打包为 zip 上传到中心服务。
5. 由中心服务按“脚本名 + 版本号”维护和下发脚本。

## 统一约定

1. 默认使用中文文档、中文注释、中文提交信息。
2. 新脚本优先基于统一脚手架创建，不直接复制旧脚本目录。
3. 关键步骤统一上报进度，避免只依赖最终成功/失败结果。
