import * as runtime from "./runtime";

import type { LoggerLike } from "../types/runtime";

interface HeartbeatSchedulerOptions {
  intervalMS: number;
  logger?: LoggerLike;
  onTick: () => void;
}

interface HeartbeatSchedulerHandle {
  kind: string;
  schedulerID: string;
  cancel(): void;
}

interface HeartbeatScheduler {
  kind: string;
  start(options: HeartbeatSchedulerOptions): HeartbeatSchedulerHandle;
}

type HeartbeatSchedulerMode = "interval" | "executor";

function javaType(name: string): any {
  if (typeof Java !== "undefined" && typeof Java.type === "function") {
    return Java.type(name);
  }

  const parts = String(name || "").split(".");
  let current = Packages;

  for (let index = 0; index < parts.length; index += 1) {
    current = current[parts[index]];
  }

  return current;
}

function createRunnable(runCallback: () => void): any {
  const Runnable = javaType("java.lang.Runnable");
  if (typeof JavaAdapter === "function") {
    return new JavaAdapter(Runnable, {
      run: runCallback
    });
  }
  return new Runnable({
    run: runCallback
  });
}

function createSchedulerID(prefix: string): string {
  return String(prefix || "scheduler") + "-" + Date.now().toString(36) + "-" + Math.floor(Math.random() * 100000).toString(36);
}

function createIntervalHeartbeatScheduler(): HeartbeatScheduler {
  return {
    kind: "interval",
    start(options: HeartbeatSchedulerOptions): HeartbeatSchedulerHandle {
      const intervalMS = Math.max(1000, Number(options.intervalMS || 30000));
      const handle = runtime.startInterval(function onHeartbeatTick() {
        options.onTick();
      }, intervalMS);
      const schedulerID = createSchedulerID("interval");

      return {
        kind: "interval",
        schedulerID,
        cancel(): void {
          handle.cancel();
        }
      };
    }
  };
}

function createExecutorHeartbeatScheduler(): HeartbeatScheduler {
  return {
    kind: "executor",
    start(options: HeartbeatSchedulerOptions): HeartbeatSchedulerHandle {
      if (!runtime.isAutoJsRuntime()) {
        return createIntervalHeartbeatScheduler().start(options);
      }

      const TimeUnit = javaType("java.util.concurrent.TimeUnit");
      const Executors = javaType("java.util.concurrent.Executors");
      const intervalMS = Math.max(1000, Number(options.intervalMS || 30000));
      const executor = Executors.newSingleThreadScheduledExecutor();
      const future = executor.scheduleAtFixedRate(createRunnable(function heartbeatTick() {
        options.onTick();
      }), intervalMS, intervalMS, TimeUnit.MILLISECONDS);
      const schedulerID = createSchedulerID("executor");

      return {
        kind: "executor",
        schedulerID,
        cancel(): void {
          future.cancel(false);
          executor.shutdownNow();
        }
      };
    }
  };
}

function normalizeSchedulerMode(mode?: string): HeartbeatSchedulerMode {
  const nextMode = String(mode || "").toLowerCase();
  if (nextMode === "interval") {
    return "interval";
  }
  return "executor";
}

function createHeartbeatScheduler(mode?: string, logger?: LoggerLike): HeartbeatScheduler {
  const normalizedMode = normalizeSchedulerMode(mode);
  if (normalizedMode === "interval") {
    return createIntervalHeartbeatScheduler();
  }
  return createExecutorHeartbeatScheduler();
}

export type {
  HeartbeatScheduler,
  HeartbeatSchedulerHandle,
  HeartbeatSchedulerMode,
  HeartbeatSchedulerOptions
};

export {
  createHeartbeatScheduler,
  normalizeSchedulerMode
};
