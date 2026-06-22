package agentpush

import "errors"

var ErrUsage = errors.New(UsageText())

func UsageText() string {
	return `Usage:
  mobilerpa-toolkit push-center --device <adb-device> --center-base-url <url> --script-name <name> --script-version <version> [--agent-root <path>] [--adb-path <path>] [--reset-config] [--no-run]
  mobilerpa-toolkit push-manual [--device <adb-device> | --all] --center-base-url <url> --script-name <name> --script-version <version> [--agent-root <path>] [--adb-path <path>] [--reset-config] [--no-run]
  mobilerpa-toolkit start-agent --device <adb-device> [--adb-path <path>] [--autojs-component <component>] [--remote-root <path>]
  mobilerpa-toolkit stop-agent --device <adb-device> [--adb-path <path>] [--remote-root <path>]

Subcommands:
  push-center   Non-interactive push for center service or automation.
  push-manual   Interactive or manual push for local debugging.
  start-agent   Start agent.js on a target device.
  stop-agent    Write a stop signal and let agent.js exit gracefully.
`
}
