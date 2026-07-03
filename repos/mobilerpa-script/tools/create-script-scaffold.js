"use strict";

var fs = require("fs");
var path = require("path");

function parseArguments(argv) {
    var result = {
        scriptName: "",
        version: "",
        force: false
    };

    for (var index = 2; index < argv.length; index += 1) {
        var current = String(argv[index] || "");
        if (current === "--script-name" && index + 1 < argv.length) {
            result.scriptName = String(argv[index + 1] || "").trim();
            index += 1;
            continue;
        }
        if (current === "--version" && index + 1 < argv.length) {
            result.version = String(argv[index + 1] || "").trim();
            index += 1;
            continue;
        }
        if (current === "--force") {
            result.force = true;
        }
    }

    return result;
}

function ensureArgument(value, flagName) {
    if (!value) {
        throw new Error("缺少参数: " + flagName);
    }
}

function resolveRoot() {
    return path.resolve(__dirname, "..");
}

function ensureDir(dirPath) {
    fs.mkdirSync(dirPath, { recursive: true });
}

function readTemplate(templatePath) {
    return fs.readFileSync(templatePath, "utf8");
}

function writeFileIfAllowed(filePath, content, force) {
    if (!force && fs.existsSync(filePath)) {
        throw new Error("目标文件已存在: " + filePath + "，如需覆盖请追加 --force");
    }
    ensureDir(path.dirname(filePath));
    fs.writeFileSync(filePath, content, "utf8");
}

function renderTemplate(template, variables) {
    return template
        .replace(/__SCRIPT_NAME__/g, variables.scriptName)
        .replace(/__SCRIPT_VERSION__/g, variables.version);
}

function main() {
    var options = parseArguments(process.argv);
    ensureArgument(options.scriptName, "--script-name");
    ensureArgument(options.version, "--version");

    var rootDir = resolveRoot();
    var templateDir = path.join(rootDir, "templates", "standard");
    var outputDir = path.join(rootDir, "publish", options.scriptName, options.version);

    ensureDir(outputDir);

    var variables = {
        scriptName: options.scriptName,
        version: options.version
    };

    var targets = [
        { source: "index.js.tpl", target: "index.js" },
        { source: "index_debug.js.tpl", target: "index_debug.js" },
        { source: "README.md.tpl", target: "README.md" }
    ];

    for (var index = 0; index < targets.length; index += 1) {
        var item = targets[index];
        var sourcePath = path.join(templateDir, item.source);
        var targetPath = path.join(outputDir, item.target);
        var content = renderTemplate(readTemplate(sourcePath), variables);
        writeFileIfAllowed(targetPath, content, options.force);
    }

    console.log("脚手架创建完成: " + outputDir);
 	console.log("后续建议:");
    console.log("1. 修改 index.js 中的业务逻辑");
    console.log("2. 在真机上直接运行 index_debug.js 调试");
    console.log("3. 调试通过后再打包上传到中心服务");
}

try {
    main();
} catch (error) {
    console.error("创建脚手架失败: " + String(error && error.message ? error.message : error));
    process.exit(1);
}
