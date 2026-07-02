export interface LoggerLike {
  info(message: string): void;
  warn(message: string): void;
  error(message: string): void;
}

export interface IntervalHandle {
  cancel(): void;
}

export interface RuntimeLike {
  isNodeRuntime(): boolean;
  isAutoJsRuntime(): boolean;
  createLogger(): LoggerLike;
  nowISOString(): string;
  fileExists(filePath: string): boolean;
  readTextFile(filePath: string): string;
  writeTextFile(filePath: string, content: string): void;
  removeFileIfExists(filePath: string): void;
  startInterval(callback: () => void, intervalMS: number): IntervalHandle;
  runAsync(callback: () => void): IntervalHandle | null;
  sleepMS(milliseconds: number): void;
  exitProcess(code: number): void;
}
