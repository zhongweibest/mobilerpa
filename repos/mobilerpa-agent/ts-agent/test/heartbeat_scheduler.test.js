"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const scheduler = require("../dist/lib/heartbeat_scheduler.js");

test("normalizeSchedulerMode 会归一化不同模式", function () {
  assert.equal(scheduler.normalizeSchedulerMode("interval"), "interval");
  assert.equal(scheduler.normalizeSchedulerMode("alarm_manager"), "executor");
  assert.equal(scheduler.normalizeSchedulerMode("unknown"), "executor");
  assert.equal(scheduler.normalizeSchedulerMode(""), "executor");
});

test("createHeartbeatScheduler 会返回对应 kind", function () {
  assert.equal(scheduler.createHeartbeatScheduler("interval").kind, "interval");
  assert.equal(scheduler.createHeartbeatScheduler("executor").kind, "executor");
  assert.equal(scheduler.createHeartbeatScheduler("alarm_manager").kind, "executor");
});
