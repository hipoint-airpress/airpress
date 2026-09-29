// 一次性脚本：按 merged 文件内的分库标记，拆分为 src/ 下的独立源文件。
// merged 结构：`/* assets/<原路径> */\n<该库压缩代码>\n/* assets/<原路径> */\n...`
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const MARKER = /\/\* (assets\/[^*]+?) \*\//g;

// 审计结论：sharejs 全库零调用，精简删除（见 spec §4.4）
const DROP = new Set(['assets/vendor/sharejs/sharejs.min.js']);

function split(srcRel) {
  const text = readFileSync(join(root, srcRel), 'utf8');
  const matches = [...text.matchAll(MARKER)];
  const written = [];
  for (let i = 0; i < matches.length; i++) {
    const m = matches[i];
    const start = m.index + m[0].length;
    const end = i + 1 < matches.length ? matches[i + 1].index : text.length;
    const body = text.slice(start, end).trim();
    const srcPath = m[1];
    if (DROP.has(srcPath)) { console.log('跳过(精简):', srcPath); continue; }
    if (!body) continue;
    const outPath = join(root, 'src', srcPath.replace(/^assets\//, '')); // assets/vendor/x → src/vendor/x
    mkdirSync(dirname(outPath), { recursive: true });
    writeFileSync(outPath, body + '\n');
    written.push(outPath.replace(root + '/', ''));
  }
  return written;
}

console.log('== JS 片段 ==');
console.log(split('assets/argon_js_merged.js').join('\n'));
console.log('== CSS 片段 ==');
console.log(split('assets/argon_css_merged.css').join('\n'));
