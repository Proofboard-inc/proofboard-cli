const https = require('https');
const fs = require('fs');
const path = require('path');
const os = require('os');
const { spawn } = require('child_process');

const DEFAULT_VERSION = 'v1.8.10';
const DEFAULT_RELEASES_URL = 'https://releases.proofboard.io/latest.json';

function getBinaryName(platform = os.type(), arch = os.arch()) {
    let osName = '';
    if (platform === 'Darwin') {
        osName = 'darwin';
    } else if (platform === 'Linux') {
        osName = 'linux';
    } else if (platform === 'Windows_NT') {
        osName = 'windows';
    } else {
        throw new Error(`Unsupported OS: ${platform}`);
    }

    let archName = '';
    if (arch === 'x64') {
        archName = 'amd64';
    } else if (arch === 'arm64') {
        archName = 'arm64';
    } else {
        throw new Error(`Unsupported architecture: ${arch}`);
    }

    if (archName === 'arm64' && osName !== 'darwin') {
        throw new Error(`Unsupported platform: ${osName}-${archName}`);
    }

    return `proofboard-${osName}-${archName}${osName === 'windows' ? '.exe' : ''}`;
}

function fetchText(url) {
    return new Promise((resolve) => {
        const req = https.get(url, (res) => {
            let data = '';
            res.on('data', (chunk) => {
                data += chunk;
            });
            res.on('end', () => resolve({ statusCode: res.statusCode, data }));
        });
        req.on('error', () => resolve({ statusCode: 0, data: '' }));
        req.end();
    });
}

async function getLatestRelease(releasesUrl = DEFAULT_RELEASES_URL) {
    const response = await fetchText(releasesUrl);
    if (response.statusCode === 200) {
        try {
            const parsed = JSON.parse(response.data);
            return parsed.version || DEFAULT_VERSION;
        } catch (err) {
            return DEFAULT_VERSION;
        }
    }
    return DEFAULT_VERSION;
}

function downloadBinary(url, dest) {
    return new Promise((resolve, reject) => {
        const file = fs.createWriteStream(dest);
        const req = https.get(url, (res) => {
            if (res.statusCode === 301 || res.statusCode === 302) {
                const location = res.headers.location;
                if (!location) {
                    reject(new Error('Missing redirect location'));
                    return;
                }
                file.close();
                fs.unlink(dest, () => {});
                downloadBinary(location, dest).then(resolve).catch(reject);
                return;
            }
            if (res.statusCode !== 200) {
                file.close();
                fs.unlink(dest, () => {});
                reject(new Error(`Failed to download binary: ${res.statusCode}`));
                return;
            }
            res.pipe(file);
            file.on('finish', () => {
                file.close();
                resolve();
            });
        });
        req.on('error', (err) => {
            file.close();
            fs.unlink(dest, () => {});
            reject(err);
        });
    });
}

async function ensureBinary(options = {}) {
    const version = options.version || await getLatestRelease(options.releasesUrl);
    const binaryName = options.binaryName || getBinaryName(options.platform, options.arch);
    const cacheDir = options.cacheDir || path.join(os.homedir(), '.proofboard', 'bin', version);
    const binaryPath = path.join(cacheDir, binaryName);
    const downloadUrl = options.downloadUrl || `https://releases.proofboard.io/${version}/${binaryName}`;
    const fallbackUrl = options.fallbackUrl || `https://github.com/Proofboard-inc/proofboard-cli/releases/download/${version}/${binaryName}`;

    if (!fs.existsSync(cacheDir)) {
        fs.mkdirSync(cacheDir, { recursive: true });
    }

    if (!fs.existsSync(binaryPath)) {
        try {
            await downloadBinary(downloadUrl, binaryPath);
        } catch (err) {
            await downloadBinary(fallbackUrl, binaryPath);
        }
        fs.chmodSync(binaryPath, 0o755);
    }

    return { binaryPath, version, binaryName };
}

function run(args = [], options = {}) {
    return ensureBinary(options).then(({ binaryPath }) => new Promise((resolve, reject) => {
        const child = spawn(binaryPath, args, {
            stdio: options.stdio || 'inherit',
            env: options.env || process.env,
        });

        child.on('error', reject);
        child.on('close', (code) => resolve(code));
    }));
}

module.exports = {
    DEFAULT_VERSION,
    getBinaryName,
    getLatestRelease,
    downloadBinary,
    ensureBinary,
    run,
};

if (require.main === module) {
    run(process.argv.slice(2)).then((code) => {
        process.exit(code);
    }).catch((err) => {
        console.error('Error running Proofboard Career Agent:', err.message);
        process.exit(1);
    });
}
