#!/usr/bin/env node

const https = require('https');
const fs = require('fs');
const path = require('path');
const os = require('os');
const { spawn } = require('child_process');

async function getLatestRelease() {
    return new Promise((resolve) => {
        const req = https.get('https://releases.proofboard.io/latest.json', (res) => {
            let data = '';
            res.on('data', chunk => data += chunk);
            res.on('end', () => {
                if (res.statusCode === 200) {
                    try {
                        const parsed = JSON.parse(data);
                        return resolve(parsed.version || 'v1.8.0');
                    } catch (e) {
                        // ignore
                    }
                }
                resolve('v1.8.0');
            });
        });
        req.on('error', () => resolve('v1.8.0'));
        req.end();
    });
}

function getBinaryName() {
    const type = os.type();
    const arch = os.arch();

    let osName = '';
    if (type === 'Darwin') {
        osName = 'darwin';
    } else if (type === 'Linux') {
        osName = 'linux';
    } else if (type === 'Windows_NT') {
        osName = 'windows';
    } else {
        throw new Error(`Unsupported OS: ${type}`);
    }

    let archName = '';
    if (arch === 'x64') {
        archName = 'amd64';
    } else if (arch === 'arm64') {
        archName = 'arm64';
    } else {
        throw new Error(`Unsupported architecture: ${arch}`);
    }

    return `proofboard-${osName}-${archName}${osName === 'windows' ? '.exe' : ''}`;
}

async function downloadBinary(url, dest, isFallback = false) {
    return new Promise((resolve, reject) => {
        const file = fs.createWriteStream(dest);
        const req = https.get(url, (res) => {
            if (res.statusCode === 301 || res.statusCode === 302) {
                return downloadBinary(res.headers.location, dest, isFallback).then(resolve).catch(reject);
            }
            if (res.statusCode !== 200) {
                if (!isFallback) {
                    return reject(new Error(`Failed to download binary: ${res.statusCode}`));
                }
                return reject(new Error(`Fallback failed: ${res.statusCode}`));
            }
            res.pipe(file);
            file.on('finish', () => {
                file.close();
                resolve();
            });
        });
        req.on('error', (err) => {
            fs.unlink(dest, () => {});
            reject(err);
        });
    });
}

async function main() {
    try {
        const binaryName = getBinaryName();
        const latestVersion = await getLatestRelease();
        const downloadUrl = `https://releases.proofboard.io/${latestVersion}/${binaryName}`;
        const githubFallbackUrl = `https://github.com/Proofboard-inc/proofboard-cli/releases/download/${latestVersion}/${binaryName}`;
        
        const binDir = path.join(os.homedir(), '.proofboard', 'bin');
        if (!fs.existsSync(binDir)) {
            fs.mkdirSync(binDir, { recursive: true });
        }

        const binPath = path.join(binDir, binaryName);

        if (!fs.existsSync(binPath)) {
            console.log(`Downloading Proofboard CLI ${latestVersion}...`);
            try {
                await downloadBinary(downloadUrl, binPath);
            } catch (err) {
                console.log('Download from releases.proofboard.io failed, falling back to GitHub...');
                await downloadBinary(githubFallbackUrl, binPath, true);
            }
            fs.chmodSync(binPath, 0o755);
        }

        const args = process.argv.slice(2);
        const child = spawn(binPath, args, { stdio: 'inherit' });
        
        child.on('close', (code) => {
            process.exit(code);
        });
    } catch (err) {
        console.error('Error running Proofboard CLI via npx:', err.message);
        process.exit(1);
    }
}

main();
