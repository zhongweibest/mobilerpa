"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const agent = require("../dist/agent.js");

test("parseCLIArgs 能正确解析中心地址与布尔参数", function () {
  const result = agent.parseCLIArgs([
    "--center", "http://127.0.0.1:18080",
    "--config", "D:/runtime/config.json",
    "--dry-run",
    "--skip-ws"
  ]);

  assert.deepEqual(result, {
    center: "http://127.0.0.1:18080",
    config: "D:/runtime/config.json",
    dryRun: true,
    skipWS: true
  });
});

test("buildRegisterPayload 会写入 device_link_sn", function () {
  const payload = agent.buildRegisterPayload("agent_xxx", {
    device_name: "Pixel",
    brand: "Google",
    model: "Pixel 8",
    android_id: "android-id-001",
    adb_serial: "serial-a"
  }, "device-link-001");

  assert.equal(payload.agent_uuid, "agent_xxx");
  assert.equal(payload.device_link_sn, "device-link-001");
  assert.equal(payload.android_id, "android-id-001");
});

test("mergeBootstrapConfig 只覆盖引导配置相关字段", function () {
  const merged = agent.mergeBootstrapConfig({
    center_base_url: "http://old",
    agent_uuid: "agent_1",
    device_id: "device_1",
    device_link_sn: "old-sn",
    websocket: {
      enabled: true,
      heartbeat_interval_ms: 30000,
      reconnect_enabled: true,
      reconnect_initial_delay_ms: 3000,
      reconnect_max_delay_ms: 60000,
      reconnect_backoff_multiplier: 2
    }
  }, {
    center_base_url: "http://new",
    device_link_sn: "new-sn",
    websocket: {
      enabled: false,
      heartbeat_interval_ms: 10000
    }
  });

  assert.equal(merged.center_base_url, "http://new");
  assert.equal(merged.device_link_sn, "new-sn");
  assert.equal(merged.agent_uuid, "agent_1");
  assert.equal(merged.device_id, "device_1");
  assert.equal(merged.websocket.enabled, false);
  assert.equal(merged.websocket.heartbeat_interval_ms, 10000);
  assert.equal(merged.websocket.reconnect_enabled, true);
});

test("acquireRuntimeLock 在未知 engine_id 但锁未过期时阻止重复启动", function () {
  const messages = [];
  const logger = {
    info(message) {
      messages.push(["info", message]);
    },
    warn(message) {
      messages.push(["warn", message]);
    },
    error() {}
  };

  const store = {
    runtimeLockPath: "D:/tmp/agent.lock.json"
  };

  const runtime = require("../dist/lib/runtime.js");
  const originalRead = runtime.readTextFile;
  const originalWrite = runtime.writeTextFile;
  const originalRemove = runtime.removeFileIfExists;

  runtime.readTextFile = function () {
    return JSON.stringify({
      engine_id: "",
      acquired_at: "2026-07-03T00:00:00.000Z",
      updated_at: new Date().toISOString()
    });
  };
  runtime.writeTextFile = function () {};
  runtime.removeFileIfExists = function () {};

  try {
    const handle = agent.acquireRuntimeLock(store, logger);
    assert.equal(handle.alreadyRunning, true);
    assert.equal(messages.some(function (item) {
      return item[0] === "warn" && String(item[1]).indexOf("未过期的 Agent 运行锁") >= 0;
    }), true);
  } finally {
    runtime.readTextFile = originalRead;
    runtime.writeTextFile = originalWrite;
    runtime.removeFileIfExists = originalRemove;
  }
});

test("acquireRuntimeLock 在相同 engine_id 但锁未过期时也阻止重复启动", function () {
  const messages = [];
  const logger = {
    info(message) {
      messages.push(["info", message]);
    },
    warn(message) {
      messages.push(["warn", message]);
    },
    error() {}
  };

  const store = {
    runtimeLockPath: "D:/tmp/agent.lock.json"
  };

  const runtime = require("../dist/lib/runtime.js");
  const originalRead = runtime.readTextFile;
  const originalWrite = runtime.writeTextFile;
  const originalRemove = runtime.removeFileIfExists;

  runtime.readTextFile = function () {
    return JSON.stringify({
      engine_id: "same-engine",
      acquired_at: "2026-07-03T00:00:00.000Z",
      updated_at: new Date().toISOString()
    });
  };
  runtime.writeTextFile = function () {};
  runtime.removeFileIfExists = function () {};

  try {
    const handle = agent.acquireRuntimeLock(store, logger);
    assert.equal(handle.alreadyRunning, true);
    assert.equal(messages.some(function (item) {
      return item[0] === "warn" && String(item[1]).indexOf("未过期的 Agent 运行锁") >= 0;
    }), true);
  } finally {
    runtime.readTextFile = originalRead;
    runtime.writeTextFile = originalWrite;
    runtime.removeFileIfExists = originalRemove;
  }
});
