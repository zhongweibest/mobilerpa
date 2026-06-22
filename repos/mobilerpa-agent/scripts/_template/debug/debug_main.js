const script = require("../src/main");

script.run({
    mode: "debug",
    task_id: "debug_task_template",
    device_id: "debug_device",
    script_name: "template_script",
    script_version: "dev",
    trace_id: "trace_template_001",
    params: {},
    timeouts: {
        task_timeout_ms: 600000,
        step_timeout_ms: 60000
    }
});
