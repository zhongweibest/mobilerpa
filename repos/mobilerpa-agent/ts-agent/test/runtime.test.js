"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const runtime = require("../dist/lib/runtime.js");

test("createStableAgentUUID 在相同 android_id 下保持稳定", function () {
  const deviceInfo = {
    device_name: "Pixel",
    brand: "Google",
    model: "Pixel 8",
    android_id: "android-id-001",
    adb_serial: "serial-a"
  };

  const first = runtime.createStableAgentUUID(deviceInfo);
  const second = runtime.createStableAgentUUID(deviceInfo);

  assert.equal(first, second);
  assert.match(first, /^agent_/);
});

test("createStableAgentUUID 在不同 android_id 下生成不同结果", function () {
  const first = runtime.createStableAgentUUID({
    device_name: "A",
    brand: "B",
    model: "C",
    android_id: "android-id-001",
    adb_serial: "serial-a"
  });
  const second = runtime.createStableAgentUUID({
    device_name: "A",
    brand: "B",
    model: "C",
    android_id: "android-id-002",
    adb_serial: "serial-a"
  });

  assert.notEqual(first, second);
});
