import { existsSync } from 'node:fs';
import { spawn } from 'node:child_process';
const go = process.platform === 'win32' && existsSync('.tools/go/bin/go.exe') ? '.tools/go/bin/go.exe' : 'go';
const child = spawn(go, ['run', './cmd/wgsimple', '--mode', 'demo', '--listen', '127.0.0.1:8787', '--data-dir', '.data/demo'], { stdio: 'inherit', env: process.env });
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => child.kill(signal));
child.on('exit', code => process.exit(code ?? 0));
