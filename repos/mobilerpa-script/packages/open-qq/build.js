"use strict";

var buildScriptPackage = require("../../tools/build-script-package").buildScriptPackage;

buildScriptPackage(__dirname).catch(function (error) {
    console.error("构建脚本失败: " + String(error && error.stack ? error.stack : error));
    process.exit(1);
});
