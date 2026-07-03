"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const taskRunner = require("../dist/lib/task_runner.js");

test("isSafeRelativePath 允许普通相对路径", function () {
  assert.equal(taskRunner.isSafeRelativePath("index.js"), true);
  assert.equal(taskRunner.isSafeRelativePath("sub/dir/index.js"), true);
});

test("isSafeRelativePath 拒绝越界路径", function () {
  assert.equal(taskRunner.isSafeRelativePath("../index.js"), false);
  assert.equal(taskRunner.isSafeRelativePath("/root/index.js"), false);
  assert.equal(taskRunner.isSafeRelativePath("./index.js"), false);
});

test("resolveScriptModule 按 scripts 目录组织版本入口", function () {
  const resolved = taskRunner.resolveScriptModule({
    script_name: "open_qq",
    script_version: "v0.1.2"
  });

  assert.match(resolved.modulePath, /scripts\/open_qq\/v0\.1\.2\/index\.js$/);
  assert.match(resolved.manifestPath, /scripts\/open_qq\/v0\.1\.2\/manifest\.json$/);
  assert.equal(resolved.entryLabel, "scripts/open_qq/v0.1.2/index.js");
});

test("normalizeModulePath 会折叠 . 和 ..", function () {
  assert.equal(
    taskRunner.normalizeModulePath("scripts/open_qq/v0.1.2/../v0.1.2/./index.js"),
    "scripts/open_qq/v0.1.2/index.js"
  );
});
