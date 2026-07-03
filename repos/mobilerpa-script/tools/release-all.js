"use strict";

var fs = require("fs");
var path = require("path");
var childProcess = require("child_process");

var repoRoot = path.resolve(__dirname, "..");
var packagesRoot = path.resolve(repoRoot, "packages");

function listPackageDirs() {
    return fs.readdirSync(packagesRoot)
        .map(function (name) {
            return path.join(packagesRoot, name);
        })
        .filter(function (fullPath) {
            return fs.statSync(fullPath).isDirectory() && fs.existsSync(path.join(fullPath, "package.json"));
        });
}

function main() {
    listPackageDirs().forEach(function (packageDir) {
        if (!fs.existsSync(path.join(packageDir, "script.config.json"))) {
            return;
        }

        console.log("开始发布脚本包: " + packageDir);
        childProcess.execFileSync("cmd.exe", ["/c", "npm", "run", "build"], {
            cwd: packageDir,
            stdio: "inherit"
        });
    });
}

main();
