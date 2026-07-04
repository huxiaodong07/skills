#!/usr/bin/env node
"use strict";

const { spawnSync } = require("node:child_process");
const path = require("node:path");

const targets = {
  "win32-x64": {
    packageName: "@xiaodonghu/skills-win32-x64",
    binary: path.join("bin", "skills.exe")
  }
};

const key = `${process.platform}-${process.arch}`;
const target = targets[key];

if (!target) {
  console.error(`@xiaodonghu/skills does not provide a binary for ${key}.`);
  console.error("Current published package supports win32-x64 only.");
  process.exit(1);
}

let executable;
try {
  const packageJson = require.resolve(`${target.packageName}/package.json`);
  executable = path.join(path.dirname(packageJson), target.binary);
} catch (error) {
  console.error(`Cannot find ${target.packageName}.`);
  console.error("Reinstall with optional dependencies enabled:");
  console.error("  npm install -g @xiaodonghu/skills --include=optional");
  process.exit(1);
}

const result = spawnSync(executable, process.argv.slice(2), {
  stdio: "inherit"
});

if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}

process.exit(result.status === null ? 1 : result.status);

