/**
 * 一次性校验脚本：检查 ember 模板里用到的所有 t('...') 键是否在语言文件中存在。
 * 缺失的键在运行时不会报错，只会显示键名本身，因此必须静态检查。
 */
import { readFile, readdir } from 'node:fs/promises';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const SRC = resolve(dirname(fileURLToPath(import.meta.url)), '..', 'src');
const TEMPLATE_DIR = join(SRC, 'templates', 'ember');

async function collectFiles(dir, out = []) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) await collectFiles(full, out);
    else if (entry.name.endsWith('.vue')) out.push(full);
  }
  return out;
}

const locales = {};
for (const locale of ['zh-CN', 'zh-TW', 'en-US']) {
  locales[locale] = JSON.parse(await readFile(join(SRC, 'i18n', 'locales', `${locale}.json`), 'utf8'));
}

function hasKey(obj, path) {
  return path.split('.').reduce((acc, part) => (acc && typeof acc === 'object' ? acc[part] : undefined), obj) !== undefined;
}

const files = await collectFiles(TEMPLATE_DIR);
const missing = new Map();

for (const file of files) {
  const source = await readFile(file, 'utf8');
  // 匹配 t('xxx') / t("xxx")，忽略带变量的动态键
  const matches = source.matchAll(/\bt\(\s*['"]([a-zA-Z0-9_.]+)['"]/g);
  for (const match of matches) {
    const key = match[1];
    if (!hasKey(locales['zh-CN'], key)) {
      const rel = file.slice(SRC.length + 1).replace(/\\/g, '/');
      if (!missing.has(key)) missing.set(key, []);
      if (!missing.get(key).includes(rel)) missing.get(key).push(rel);
    }
  }
}

const total = files.length;
if (missing.size === 0) {
  process.stdout.write(`\n\x1b[32m✓\x1b[0m ${total} 个模板文件，全部 i18n 键均存在\n\n`);
} else {
  process.stdout.write(`\n\x1b[31m✗ 发现 ${missing.size} 个缺失的 i18n 键：\x1b[0m\n\n`);
  for (const [key, where] of [...missing.entries()].sort()) {
    process.stdout.write(`  ${key}\n      ← ${where.join(', ')}\n`);
  }
  process.stdout.write('\n');
  process.exitCode = 1;
}

// 顺带检查三个语言的键结构是否一致（ember 命名空间）
const emberKeys = (obj, prefix = '') =>
  Object.entries(obj ?? {}).flatMap(([k, v]) =>
    v && typeof v === 'object' ? emberKeys(v, `${prefix}${k}.`) : [`${prefix}${k}`],
  );

const base = emberKeys(locales['zh-CN'].ember).sort();
for (const locale of ['zh-TW', 'en-US']) {
  const other = emberKeys(locales[locale].ember).sort();
  const diff = base.filter((k) => !other.includes(k)).concat(other.filter((k) => !base.includes(k)));
  process.stdout.write(
    diff.length === 0
      ? `  \x1b[32m✓\x1b[0m ${locale} 的 ember 命名空间与 zh-CN 一致（${base.length} 个键）\n`
      : `  \x1b[31m✗\x1b[0m ${locale} 的 ember 键不一致：${diff.join(', ')}\n`,
  );
  if (diff.length > 0) process.exitCode = 1;
}
process.stdout.write('\n');
