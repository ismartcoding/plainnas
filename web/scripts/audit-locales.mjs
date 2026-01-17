import fs from 'node:fs/promises';
import vm from 'node:vm';
import ts from 'typescript';

const LOCALES_DIR = new URL('../src/locales/', import.meta.url);

function evalTsDefaultExport(tsCode, filename) {
    const js = ts.transpileModule(tsCode, {
        compilerOptions: {
            target: ts.ScriptTarget.ES2020,
            module: ts.ModuleKind.CommonJS,
            esModuleInterop: true,
            strict: false,
        },
        fileName: filename,
    }).outputText;

    const sandbox = {
        exports: {},
        module: { exports: {} },
        require: () => {
            throw new Error('require() is not supported in locale files');
        },
    };

    vm.runInNewContext(js, sandbox, { filename });
    return sandbox.module.exports?.default ?? sandbox.exports?.default;
}

function flatten(obj, prefix = '', out = new Map()) {
    if (!obj || typeof obj !== 'object') return out;
    for (const [key, value] of Object.entries(obj)) {
        const next = prefix ? `${prefix}.${key}` : key;
        if (value && typeof value === 'object' && !Array.isArray(value)) {
            flatten(value, next, out);
        } else {
            out.set(next, value);
        }
    }
    return out;
}

function looksLikeEnglishSentence(text) {
    if (typeof text !== 'string') return false;
    // Heuristic: contains multiple ASCII words.
    return /[A-Za-z]{3,}/.test(text) && /\s/.test(text);
}

const IGNORE_SAME_AS_EN_KEYS = new Set([
    'app_name',
    // Proper noun/tech label; may legitimately stay in English.
    'opengl_es',
]);

const files = (await fs.readdir(LOCALES_DIR)).filter((f) => f.endsWith('.ts'));
files.sort();

const enFile = files.find((f) => f.toLowerCase() === 'en-us.ts');
if (!enFile) {
    console.error('Could not find en-US.ts baseline');
    process.exitCode = 1;
    process.exit();
}

const baselineTs = await fs.readFile(new URL(enFile, LOCALES_DIR), 'utf8');
const baselineObj = evalTsDefaultExport(baselineTs, enFile);
const baseline = flatten(baselineObj);
const baselineKeys = new Set(baseline.keys());

let hasIssues = false;

for (const file of files) {
    const tsCode = await fs.readFile(new URL(file, LOCALES_DIR), 'utf8');
    const obj = evalTsDefaultExport(tsCode, file);
    const flat = flatten(obj);

    const missing = [];
    const extra = [];
    const englishPlaceholders = [];

    for (const key of baselineKeys) {
        if (!flat.has(key)) missing.push(key);
    }
    for (const key of flat.keys()) {
        if (!baselineKeys.has(key)) extra.push(key);
    }

    for (const [key, value] of flat.entries()) {
        if (IGNORE_SAME_AS_EN_KEYS.has(key)) continue;
        const en = baseline.get(key);
        if (typeof value === 'string' && typeof en === 'string' && value === en && looksLikeEnglishSentence(value)) {
            englishPlaceholders.push(key);
        }
    }

    if (missing.length || extra.length || (file !== enFile && englishPlaceholders.length)) {
        hasIssues = true;
        console.log(`\n${file}`);
        if (missing.length) console.log(`  missing: ${missing.length}`);
        if (extra.length) console.log(`  extra: ${extra.length}`);
        if (file !== enFile && englishPlaceholders.length) console.log(`  same-as-en (english-looking): ${englishPlaceholders.length}`);

        const show = (label, arr) => {
            if (!arr.length) return;
            console.log(`  ${label}:`);
            for (const k of arr.slice(0, 50)) console.log(`    - ${k}`);
            if (arr.length > 50) console.log(`    … +${arr.length - 50} more`);
        };

        show('missing keys', missing);
        show('extra keys', extra);
        if (file !== enFile) show('english-looking keys', englishPlaceholders);
    }
}

if (!hasIssues) {
    console.log('All locales match en-US.ts keys, and no english-looking placeholders found.');
}
