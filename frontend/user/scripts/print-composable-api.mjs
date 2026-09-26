/** 开发辅助：打印某个 composable 返回的全部字段名 */
import { readFile, readdir } from 'node:fs/promises';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const SRC = resolve(dirname(fileURLToPath(import.meta.url)), '..', 'src');
const targets = process.argv.slice(2);

function extractReturnBody(source) {
  const index = source.lastIndexOf('return {');
  if (index === -1) return null;
  let depth = 0;
  let started = false;
  for (let i = index + 7; i < source.length; i += 1) {
    const ch = source[i];
    if (ch === '{') { depth += 1; started = true; }
    else if (ch === '}') {
      depth -= 1;
      if (started && depth === 0) return source.slice(index + 7, i + 1);
    }
  }
  return null;
}

function extractKeys(body) {
  const inner = body.slice(1, -1).replace(/\/\/[^\n\r]*/g, '').replace(/\/\*[\s\S]*?\*\//g, '');
  const keys = [];
  let depth = 0;
  let buffer = '';
  const flush = () => {
    const trimmed = buffer.trim().replace(/,\s*$/, '').trim();
    buffer = '';
    if (!trimmed || trimmed.startsWith('...')) return;
    const m = trimmed.match(/^([A-Za-z_$][\w$]*)/);
    if (m) keys.push(m[1]);
  };
  for (const ch of inner) {
    if ('{[('.includes(ch)) depth += 1;
    if ('}])'.includes(ch)) depth -= 1;
    if (ch === ',' && depth === 0) { flush(); continue; }
    buffer += ch;
  }
  flush();
  return keys;
}

const files = [];
const walk = async (dir) => {
  for (const e of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, e.name);
    if (e.isDirectory()) await walk(full);
    else if (e.name.endsWith('.ts')) files.push(full);
  }
};
await walk(join(SRC, 'composables'));
await walk(join(SRC, 'utils'));

const index = new Map();
for (const file of files) {
  const source = await readFile(file, 'utf8');
  const re = /export\s+(?:function\s+([A-Za-z_$][\w$]*)|const\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?\()/g;
  const decls = [...source.matchAll(re)];
  decls.forEach((d, i) => {
    const name = d[1] || d[2];
    if (!name || index.has(name)) return;
    index.set(name, source.slice(d.index, i + 1 < decls.length ? decls[i + 1].index : source.length));
  });
}

for (const name of targets) {
  const segment = index.get(name);
  if (!segment) { process.stdout.write(`${name}: (未找到)\n`); continue; }
  const body = extractReturnBody(segment);
  if (!body) { process.stdout.write(`${name}: (无 return 块)\n`); continue; }
  const keys = extractKeys(body);
  process.stdout.write(`${name} (${keys.length}):\n  ${keys.join(', ')}\n\n`);
}
