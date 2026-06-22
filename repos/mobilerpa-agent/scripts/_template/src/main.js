module.exports = {
    run(context) {
        const currentStep = "template_step";
        log("template script start");
        log(JSON.stringify({
            task_id: context && context.task_id,
            device_id: context && context.device_id,
            mode: context && context.mode,
            step: currentStep
        }));

        return {
            success: true,
            code: "OK",
            message: "template finished",
            step: currentStep,
            data: {},
            artifacts: {
                screenshots: [],
                dumps: []
            }
        };
    }
};
