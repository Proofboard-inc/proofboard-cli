const https = require('https');
const fs = require('fs');
const path = require('path');
const os = require('os');
const crypto = require('crypto');
const { spawn } = require('child_process');

const DEFAULT_VERSION = 'v1.8.18';
const DEFAULT_RELEASES_URL = 'https://releases.proofboard.io/latest.json';
const RELEASE_PUBLIC_KEY = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEdYPsxqaryQ9bQI3G3hQpsmyrTGs0
nKxvQXQC+nAK+EsNF6VEofCYuX42bTeooKLR1Ol+Eh3NhWErh4tfSkH1mA==
-----END PUBLIC KEY-----`;

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

function fetchBuffer(url, redirectsRemaining = 5) {
    return new Promise((resolve) => {
        let parsedURL;
        try {
            parsedURL = new URL(url);
        } catch (err) {
            resolve({ statusCode: 0, data: Buffer.alloc(0) });
            return;
        }
        if (parsedURL.protocol !== 'https:') {
            resolve({ statusCode: 0, data: Buffer.alloc(0) });
            return;
        }
        const req = https.get(parsedURL, (res) => {
            if ([301, 302, 303, 307, 308].includes(res.statusCode) && redirectsRemaining > 0) {
                const location = res.headers.location;
                res.resume();
                if (!location) {
                    resolve({ statusCode: res.statusCode, data: Buffer.alloc(0) });
                    return;
                }
                fetchBuffer(new URL(location, url).toString(), redirectsRemaining - 1).then(resolve);
                return;
            }
            const chunks = [];
            res.on('data', (chunk) => chunks.push(Buffer.from(chunk)));
            res.on('end', () => resolve({ statusCode: res.statusCode, data: Buffer.concat(chunks) }));
        });
        req.on('error', () => resolve({ statusCode: 0, data: Buffer.alloc(0) }));
        req.end();
    });
}

async function getLatestReleaseInfo(releasesUrl = DEFAULT_RELEASES_URL) {
    const response = await fetchBuffer(releasesUrl);
    if (response.statusCode === 200) {
        try {
            const parsed = JSON.parse(response.data.toString('utf8'));
            const version = parsed.version || DEFAULT_VERSION;
            const releaseTag = version.startsWith('v') ? version : `v${version}`;
            return {
                version,
                url: parsed.url || `https://releases.proofboard.io/${releaseTag}`,
            };
        } catch (err) {
            // Fall through to the pinned release.
        }
    }
    return {
        version: DEFAULT_VERSION,
        url: `https://releases.proofboard.io/${DEFAULT_VERSION}`,
    };
}

async function getLatestRelease(releasesUrl = DEFAULT_RELEASES_URL) {
    return (await getLatestReleaseInfo(releasesUrl)).version;
}

async function downloadBinary(url, dest) {
    const response = await fetchBuffer(url);
    if (response.statusCode !== 200) {
        throw new Error(`Failed to download binary: ${response.statusCode}`);
    }
    fs.writeFileSync(dest, response.data, { mode: 0o600 });
}

function verifyBinarySignature(binaryPath, signaturePath, publicKey = RELEASE_PUBLIC_KEY) {
    const valid = crypto.verify(
        'sha256',
        fs.readFileSync(binaryPath),
        { key: publicKey, dsaEncoding: 'der' },
        fs.readFileSync(signaturePath),
    );
    if (!valid) {
        throw new Error('Proofboard release signature verification failed');
    }
}

async function ensureBinary(options = {}) {
    const release = options.version ? {
        version: options.version,
        url: options.releaseBaseUrl || `https://releases.proofboard.io/${options.version.startsWith('v') ? options.version : `v${options.version}`}`,
    } : await getLatestReleaseInfo(options.releasesUrl);
    const version = release.version;
    const binaryName = options.binaryName || getBinaryName(options.platform, options.arch);
    const cacheDir = options.cacheDir || path.join(os.homedir(), '.proofboard', 'bin', version);
    const binaryPath = path.join(cacheDir, binaryName);
    const signaturePath = `${binaryPath}.sig`;
    const downloadUrl = options.downloadUrl || `${release.url.replace(/\/$/, '')}/${binaryName}`;
    const signatureUrl = options.signatureUrl || `${downloadUrl}.sig`;

    if (!fs.existsSync(cacheDir)) {
        fs.mkdirSync(cacheDir, { recursive: true });
    }

    let downloadRequired = !fs.existsSync(binaryPath) || !fs.existsSync(signaturePath);
    if (!downloadRequired) {
        try {
            verifyBinarySignature(binaryPath, signaturePath, options.publicKey);
        } catch (err) {
            fs.rmSync(binaryPath, { force: true });
            fs.rmSync(signaturePath, { force: true });
            downloadRequired = true;
        }
    }

    if (downloadRequired) {
        try {
            await downloadBinary(downloadUrl, binaryPath);
            await downloadBinary(signatureUrl, signaturePath);
            verifyBinarySignature(binaryPath, signaturePath, options.publicKey);
        } catch (err) {
            fs.rmSync(binaryPath, { force: true });
            fs.rmSync(signaturePath, { force: true });
            throw err;
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
    getLatestReleaseInfo,
    downloadBinary,
    verifyBinarySignature,
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
