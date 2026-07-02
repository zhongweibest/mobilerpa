const fs = require("fs");
const path = require("path");

const projectRoot = path.resolve(__dirname, "..");
const distRoot = path.join(projectRoot, "dist");
const entryPath = path.join(distRoot, "agent.js");
const releaseRoot = path.join(projectRoot, "release");
const releaseEntryPath = path.join(releaseRoot, "agent.js");
const releaseConfigPath = path.join(releaseRoot, "config.example.json");

function readText(filePath) {
  return fs.readFileSync(filePath, "utf8");
}

function ensureDir(dirPath) {
  fs.mkdirSync(dirPath, { recursive: true });
}

function escapeForTemplateLiteral(text) {
  return String(text)
    .replace(/\\/g, "\\\\")
    .replace(/`/g, "\\`")
    .replace(/\$\{/g, "\\${");
}

function collectFiles(dirPath, result) {
  const entries = fs.readdirSync(dirPath, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(dirPath, entry.name);
    if (entry.isDirectory()) {
      if (entry.name === "types") {
        continue;
      }
      collectFiles(fullPath, result);
      continue;
    }
    if (entry.isFile() && entry.name.endsWith(".js")) {
      result.push(fullPath);
    }
  }
}

function normalizeRelativeModuleName(filePath) {
  const relativePath = path.relative(distRoot, filePath).replace(/\\/g, "/");
  return "./" + relativePath.replace(/\.js$/, "");
}

function dirnameModuleName(moduleName) {
  const normalized = String(moduleName || "").replace(/\\/g, "/");
  const index = normalized.lastIndexOf("/");
  if (index <= 0) {
    return ".";
  }
  return normalized.slice(0, index);
}

function joinModuleName(baseName, requestName) {
  const baseParts = String(baseName || ".").split("/");
  const requestParts = String(requestName || "").split("/");
  const output = [];
  const source = baseParts.concat(requestParts);

  for (const part of source) {
    if (!part || part === ".") {
      continue;
    }
    if (part === "..") {
      if (output.length > 0) {
        output.pop();
      }
      continue;
    }
    output.push(part);
  }

  return "./" + output.join("/");
}

function replaceExportsAssignment(sourceText) {
  return sourceText
    .replace(
      /Object\.defineProperty\(exports,\s*"__esModule",\s*\{\s*value:\s*true\s*\}\);\s*/g,
      ""
    )
    .replace(/(^|[^.\w$])exports\.([A-Za-z0-9_$]+)\s*=\s*/gm, "$1module.exports.$2 = ");
}

function replaceRequires(sourceText) {
  return sourceText.replace(
    /require\("(\.\/[^"]+)"\)/g,
    function replaceLocalRequire(_all, moduleName) {
      return "__bundleRequire(\"" + moduleName + "\")";
    }
  );
}

function buildBundleSource(moduleFiles) {
  const moduleRecords = moduleFiles.map((filePath) => {
    const moduleName = normalizeRelativeModuleName(filePath);
    const moduleDir = dirnameModuleName(moduleName);
    const moduleBody = replaceRequires(replaceExportsAssignment(readText(filePath))).replace(
      /__bundleRequire\("(\.\/[^"]+)"\)/g,
      function replaceScopedRequire(_all, requestName) {
        return "__bundleRequire(\"" + joinModuleName(moduleDir, requestName) + "\")";
      }
    );
    return {
      moduleName,
      moduleBody
    };
  });

  const modulesSource = moduleRecords.map((record) => {
    return "  \"" + record.moduleName + "\": function(module, exports, __bundleRequire) {\n"
      + record.moduleBody
      + "\n  }";
  }).join(",\n");

  return "\"use strict\";\n"
    + "(function () {\n"
    + "var __bundleModules = {\n"
    + modulesSource + "\n"
    + "};\n"
    + "var __bundleCache = {};\n"
    + "function __bundleResolve(moduleName) {\n"
    + "  var normalized = String(moduleName || \"\");\n"
    + "  if (__bundleModules[normalized]) {\n"
    + "    return normalized;\n"
    + "  }\n"
    + "  if (__bundleModules[normalized + \".js\"]) {\n"
    + "    return normalized + \".js\";\n"
    + "  }\n"
    + "  throw new Error(\"bundle_module_not_found:\" + normalized);\n"
    + "}\n"
    + "function __bundleRequire(moduleName) {\n"
    + "  var resolved = __bundleResolve(moduleName);\n"
    + "  if (__bundleCache[resolved]) {\n"
    + "    return __bundleCache[resolved].exports;\n"
    + "  }\n"
    + "  var module = { exports: {} };\n"
    + "  __bundleCache[resolved] = module;\n"
    + "  __bundleModules[resolved](module, module.exports, __bundleRequire);\n"
    + "  return module.exports;\n"
    + "}\n"
    + "__bundleRequire(\"./agent\");\n"
    + "})();\n";
}

function main() {
  if (!fs.existsSync(entryPath)) {
    throw new Error("dist entry not found: " + entryPath);
  }

  const files = [];
  collectFiles(distRoot, files);

  const jsFiles = files
    .filter((filePath) => filePath !== entryPath)
    .sort(function sortPath(left, right) {
      return left.localeCompare(right);
    });

  jsFiles.push(entryPath);

  const bundleText = buildBundleSource(jsFiles);
  fs.writeFileSync(entryPath, bundleText, "utf8");

  ensureDir(releaseRoot);
  fs.writeFileSync(releaseEntryPath, bundleText, "utf8");

  const configExamplePath = path.join(projectRoot, "config.example.json");
  if (fs.existsSync(configExamplePath)) {
    fs.copyFileSync(configExamplePath, releaseConfigPath);
  }
}

main();
