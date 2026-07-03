declare function sleep(milliseconds: number): void;
declare function currentPackage(): string;
declare function toast(message: string): void;

declare const app: {
  launchPackage(packageName: string): void;
};

declare const auto: {
  service?: unknown;
};

declare const java: {
  lang?: {
    Thread?: {
      sleep?: (milliseconds: number) => void;
    };
  };
};
