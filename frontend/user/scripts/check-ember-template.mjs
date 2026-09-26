/**
 * 静态校验（不依赖 node_modules）：
 *   1. 每个 .vue 里 `const { ... } = useXxx()` 解构出的字段，是否真的在该 composable 的 return 中
 *   2. 所有本地 import（./ 与 @/ 开头）指向的文件是否存在
 *   3. 模板标签是否配平（粗检）
 *
 * 这三类错误在运行时表现为 undefined / 白屏，且 vue-tsc 在容器外不可用，因此必须静态兜住。
 */
import { readFile, readdir, stat } from 'node:fs/promises';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const SRC = resolve(dirname(fileURLToPath(import.meta.url)), '..', 'src');
const TEMPLATE_DIR = join(SRC, 'templates', 'ember');

let failures = 0;

/* -------------------------------------------------------------------------- */
/* 工具                                                                        */
/* -------------------------------------------------------------------------- */

async function collectVue(dir, out = []) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) await collectVue(full, out);
    else if (entry.name.endsWith('.vue')) out.push(full);
  }
  return out;
}

/** 从源码中截取 `return { ... }` 的对象体（按大括号配平） */
function extractReturnBody(source) {
  const index = source.lastIndexOf('return {');
  if (index === -1) return null;
  let depth = 0;
  let started = false;
  for (let i = index + 'return '.length; i < source.length; i += 1) {
    const ch = source[i];
    if (ch === '{') {
      depth += 1;
      started = true;
    } else if (ch === '}') {
      depth -= 1;
      if (started && depth === 0) return source.slice(index + 'return '.length, i + 1);
    }
  }
  return null;
}

/** 从 return 对象体里提取顶层键名与展开项（忽略嵌套对象内部与行注释） */
function extractReturnParts(body) {
  // 先去掉行注释，否则注释后的第一个字段会被当成注释的一部分丢掉
  const inner = body
    .slice(1, -1)
    .replace(/\/\/[^\n\r]*/g, '')
    .replace(/\/\*[\s\S]*?\*\//g, '');

  const keys = new Set();
  const spreads = new Set();
  let depth = 0;
  let buffer = '';

  const flush = () => {
    const trimmed = buffer.trim().replace(/,\s*$/, '').trim();
    buffer = '';
    if (!trimmed) return;

    // `...helpers` 或 `...useOrderDisplayHelpers(order)`
    const spreadMatch = trimmed.match(/^\.\.\.\s*([A-Za-z_$][\w$]*)/);
    if (spreadMatch) {
      spreads.add(spreadMatch[1]);
      return;
    }

    // 形如 `name` / `name: alias` / `name() {}` / `name,`
    const match = trimmed.match(/^([A-Za-z_$][\w$]*)/);
    if (match) keys.add(match[1]);
  };

  for (const ch of inner) {
    if ('{[('.includes(ch)) depth += 1;
    if ('}])'.includes(ch)) depth -= 1;
    if (ch === ',' && depth === 0) {
      flush();
      continue;
    }
    buffer += ch;
  }
  flush();
  return { keys, spreads };
}

/* -------------------------------------------------------------------------- */
/* 校验 1：composable 解构字段                                                  */
/* -------------------------------------------------------------------------- */

// composable 可能分散在 composables/ 与 utils/ 下（如 useTheme 在 utils/theme.ts），
// 一个文件里也可能导出多个（如 useProduct.ts 同时导出 useLocalized 与 useProductLabels），
// 因此先建立「函数名 → 源码片段」索引，再取该片段里最后一个 return { ... }。
let composableIndexPromise = null;
const composableCache = new Map();

async function buildComposableIndex() {
  const files = [];

  const walk = async (current) => {
    for (const entry of await readdir(current, { withFileTypes: true })) {
      const full = join(current, entry.name);
      if (entry.isDirectory()) {
        if (entry.name === 'i18n' || entry.name === 'views' || entry.name === 'templates') continue;
        await walk(full);
      } else if (entry.name.endsWith('.ts')) {
        files.push(full);
      }
    }
  };
  await walk(join(SRC, 'composables'));
  await walk(join(SRC, 'utils'));

  const index = new Map();
  for (const file of files) {
    const source = await readFile(file, 'utf8');
    // 同时支持 `export function useX(` 与 `export const useX = (…) =>`
    const declRe =
      /export\s+(?:function\s+([A-Za-z_$][\w$]*)|const\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?\()/g;
    const decls = [...source.matchAll(declRe)];
    decls.forEach((decl, i) => {
      const name = decl[1] || decl[2];
      if (!name) return;
      const start = decl.index;
      const end = i + 1 < decls.length ? decls[i + 1].index : source.length;
      if (!index.has(name)) index.set(name, source.slice(start, end));
    });
  }
  return index;
}

/**
 * 解析某个 composable 返回的全部字段。
 * 会递归展开 `...helpers` 这类 spread（helpers 可能是另一个 composable 的返回值）。
 */
async function getComposableKeys(name, depth = 0) {
  if (composableCache.has(name)) return composableCache.get(name);
  if (depth > 3) return null;

  if (!composableIndexPromise) composableIndexPromise = buildComposableIndex();
  const index = await composableIndexPromise;

  const segment = index.get(name);
  if (!segment) return null;

  const body = extractReturnBody(segment);
  if (!body) return null;

  const { keys, spreads } = extractReturnParts(body);

  for (const spread of spreads) {
    // `const helpers = useXxx(...)` 形式：解析 useXxx 并合并
    const viaConst = segment.match(
      new RegExp(`const\\s+${spread}\\s*=\\s*(use[A-Za-z_$][\\w$]*)\\s*\\(`),
    );
    if (viaConst) {
      const sub = await getComposableKeys(viaConst[1], depth + 1);
      if (sub) for (const key of sub) keys.add(key);
      continue;
    }
    // `...useXxx(...)` 直接展开
    if (/^use[A-Z]/.test(spread)) {
      const sub = await getComposableKeys(spread, depth + 1);
      if (sub) for (const key of sub) keys.add(key);
    }
  }

  composableCache.set(name, keys);
  return keys;
}

/* -------------------------------------------------------------------------- */
/* 校验 2：本地 import 路径                                                     */
/* -------------------------------------------------------------------------- */

const IMPORT_RE = /from\s+['"]((?:\.{1,2}\/|@\/)[^'"]+)['"]/g;

async function checkImports(file, source) {
  const problems = [];
  for (const match of source.matchAll(IMPORT_RE)) {
    const spec = match[1];
    const base = spec.startsWith('@/')
      ? join(SRC, spec.slice(2))
      : resolve(dirname(file), spec);

    const candidates = [
      base,
      `${base}.ts`,
      `${base}.vue`,
      `${base}.js`,
      join(base, 'index.ts'),
      join(base, 'index.vue'),
      join(base, 'index.js'),
    ];

    let found = false;
    for (const candidate of candidates) {
      try {
        const info = await stat(candidate);
        if (info.isFile()) {
          found = true;
          break;
        }
      } catch {
        /* 继续试下一个 */
      }
    }
    if (!found) problems.push(`import 路径无效：${spec}`);
  }
  return problems;
}

/* -------------------------------------------------------------------------- */
/* 校验 3：模板标签配平（粗检）                                                  */
/* -------------------------------------------------------------------------- */

const VOID_TAGS = new Set([
  'area', 'base', 'br', 'col', 'embed', 'hr', 'img', 'input',
  'link', 'meta', 'param', 'source', 'track', 'wbr',
]);

function checkTemplateBalance(source) {
  const match = source.match(/<template>([\s\S]*)<\/template>/);
  if (!match) return ['缺少 <template> 块'];
  const html = match[1];

  // 去掉注释
  const cleaned = html.replace(/<!--[\s\S]*?-->/g, '');
  const stack = [];
  const problems = [];
  const tagRe = /<(\/?)([A-Za-z][\w.-]*)((?:"[^"]*"|'[^']*'|[^>"'])*?)(\/?)>/g;

  for (const m of cleaned.matchAll(tagRe)) {
    const [, closing, rawName, , selfClose] = m;
    const name = rawName.toLowerCase();
    if (VOID_TAGS.has(name) || selfClose === '/') continue;

    if (!closing) {
      stack.push(name);
      continue;
    }
    const last = stack.pop();
    if (last !== name) {
      problems.push(`标签未配平：</${name}> 对应 <${last ?? '空'}>`);
      if (problems.length >= 3) return problems;
    }
  }

  if (stack.length > 0) problems.push(`有 ${stack.length} 个标签未闭合：${stack.slice(-3).join(', ')}`);
  return problems;
}

/* -------------------------------------------------------------------------- */
/* 主流程                                                                      */
/* -------------------------------------------------------------------------- */

const files = await collectVue(TEMPLATE_DIR);
process.stdout.write(`\n校验 ${files.length} 个模板文件\n\n`);

for (const file of files) {
  const rel = file.slice(SRC.length + 1).replace(/\\/g, '/');
  const source = await readFile(file, 'utf8');
  const problems = [];

  // 1) composable 解构
  const destructureRe = /const\s*\{([\s\S]*?)\}\s*=\s*(use[A-Za-z]+)\(/g;
  for (const match of source.matchAll(destructureRe)) {
    const [, fieldsRaw, composable] = match;
    const keys = await getComposableKeys(composable);
    if (!keys) {
      problems.push(`${composable}() 的 return 无法解析`);
      continue;
    }
    const fields = fieldsRaw
      .split(',')
      .map((f) => f.trim().split(':')[0].trim())
      .filter((f) => f && /^[A-Za-z_$][\w$]*$/.test(f));

    const missing = fields.filter((f) => !keys.has(f));
    if (missing.length) {
      problems.push(`${composable}() 未返回：${missing.join(', ')}`);
    }
  }

  // 2) import 路径
  problems.push(...(await checkImports(file, source)));

  // 3) 模板配平
  problems.push(...checkTemplateBalance(source));

  if (problems.length === 0) {
    process.stdout.write(`  \x1b[32m✓\x1b[0m ${rel}\n`);
  } else {
    failures += 1;
    process.stdout.write(`  \x1b[31m✗\x1b[0m ${rel}\n`);
    for (const p of problems) process.stdout.write(`      ${p}\n`);
  }
}

process.stdout.write(
  failures === 0
    ? `\n\x1b[32m全部通过\x1b[0m\n\n`
    : `\n\x1b[31m${failures} 个文件有问题\x1b[0m\n\n`,
);
if (failures > 0) process.exitCode = 1;
