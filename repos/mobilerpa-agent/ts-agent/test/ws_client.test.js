"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const wsClient = require("../dist/lib/ws_client.js");

test("buildWebSocketURL 能从 http 和 https 正确转换", function () {
  assert.equal(
    wsClient.buildWebSocketURL("http://127.0.0.1:18080"),
    "ws://127.0.0.1:18080/ws"
  );
  assert.equal(
    wsClient.buildWebSocketURL("https://example.com"),
    "wss://example.com/ws"
  );
});

test("createEnvelope 会生成标准消息外壳", function () {
  const envelope = wsClient.createEnvelope("heartbeat", "req-001", "device-001", {
    ping: true
  });

  assert.equal(envelope.type, "heartbeat");
  assert.equal(envelope.request_id, "req-001");
  assert.equal(envelope.device_id, "device-001");
  assert.deepEqual(envelope.payload, {
    ping: true
  });
  assert.equal(typeof envelope.timestamp, "number");
});

test("createSessionFlagStore 支持 put / has / remove", function () {
  const store = wsClient.createSessionFlagStore();

  assert.equal(store.has("session-1"), false);
  store.put("session-1");
  assert.equal(store.has("session-1"), true);
  store.remove("session-1");
  assert.equal(store.has("session-1"), false);
});
