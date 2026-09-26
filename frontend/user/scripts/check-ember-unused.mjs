/**
 * 检查 ember 模板中的未使用 import 与未使用变量。
 *
 * 背景：tsconfig.app.json 开启了 noUnusedLocals / noUnusedParameters，
 * 任何未使用的 import 或变量都会让 `pnpm build`（vue-tsc）直接失败。
 *
 * 判定方式：标识符在「去掉 import 语句后的全文」中是否还有出现。
 */
import { readFile, readdir } from 'node:fs/promises';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const SRC = resolve(dirname(fileURLToPath(import.meta.url)), '..', 'src');
const TEMPLATE_DIR = join(SRC, 'templates', 'ember');

async function collect(dir, out = []) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) await collect(full, out);
    else if (entry.name.endsWith('.vue')) out.push(full);
  }
  return out;
}

/** 把 import 语句整段剔除，保留其余代码用于引用检测 */
function stripImports(script) {
  return script
    .split('\n')
    .filter((line) => !/^\s*import\s/.test(line) && !/^\s*\}?\s*from\s+['"]/.test(line))
    .join('\n');
}

const files = await collect(TEMPLATE_DIR);
let problemFiles = 0;

process.stdout.write(`\n检查 ${files.length} 个模板文件的未使用标识符\n\n`);

for (const file of files) {
  const rel = file.slice(SRC.length + 1).replace(/\\/g, '/');
  const source = await readFile(file, 'utf8');

  const scriptMatch = source.match(/<script setup[^>]*>([\s\S]*?)<\/script>/);
  if (!scriptMatch) continue;

  const script = scriptMatch[1];
  const template = source.slice(0, source.indexOf('<script setup'));
  const body = stripImports(script);
  const searchSpace = `${body}\n${template}`;

  // 收集 import 的标识符
  const imported = [];
  const importRe = /import\s+([\s\S]*?)\s+from\s+['"][^'"]+['"]/g;
  for (const match of script.matchAll(importRe)) {
    const clause = match[1];
    const braced = clause.match(/\{([\s\S]*)\}/);
    if (braced) {
      for (const part of braced[1].split(',')) {
        const name = part.trim().replace(/^type\s+/, '').split(/\s+as\s+/).pop().trim();
        if (/^[A-Za-z_$][\w$]*$/.test(name)) imported.push(name);
      }
    }
    const defaultMatch = clause.match(/^\s*([A-Za-z_$][\w$]*)/);
    if (defaultMatch) imported.push(defaultMatch[1]);
  }

  const unused = imported.filter((name) => {
    const re = new RegExp(`\\b${name.replace(/\$/g, '\\$')}\\b`);
    return !re.test(searchSpace);
  });

  // 检查 script setup 里声明的、既未在模板也未在脚本中二次引用的顶层 const
  const declared = [...body.matchAll(/^const\s+([A-Za-z_$][\w$]*)\s*=/gm)].map((m) => m[1]);
  const unusedVars = declared.filter((name) => {
    const re = new RegExp(`\\b${name.replace(/\$/g, '\\$')}\\b`, 'g');
    const hits = (searchSpace.match(re) || []).length;
    return hits <= 1;
  });

  const problems = [...unused.map((n) => `未使用 import：${n}`), ...unusedVars.map((n) => `未使用变量：${n}`)];

  if (problems.length === 0) {
    process.stdout.write(`  \x1b[32m✓\x1b[0m ${rel}\n`);
  } else {
    problemFiles += 1;
    process.stdout.write(`  \x1b[31m✗\x1b[0m ${rel}\n`);
    for (const p of problems) process.stdout.write(`      ${p}\n`);
  }
}

process.stdout.write(
  problemFiles === 0
    ? '\n\x1b[32m全部通过\x1b[0m（noUnusedLocals 安全）\n\n'
    : `\n\x1b[31m${problemFiles} 个文件存在问题\x1b[0m\n\n`,
);
if (problemFiles > 0) process.exitCode = 1;
