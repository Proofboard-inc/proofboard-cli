const https = require('https');
const fs = require('fs');
const path = require('path');
const os = require('os');
const crypto = require('crypto');
const { spawn } = require('child_process');

const DEFAULT_VERSION = 'v1.10.0';
const DEFAULT_RELEASES_URL = 'https://proofboard.io/latest.json';
const GITHUB_LATEST_RELEASE_URL = 'https://github.com/Proofboard-inc/proofboard-cli/releases/latest/download/latest.json';
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

    // Linux and Windows on ARM are published, so no combination is refused
    // here any more. Rejecting them described the release rather than the tool.

    return `proofboard-${osName}-${archName}${osName === 'windows' ? '.exe' : ''}`;
}

// Release assets are named for the product, matching every installer on the
// release page. getBinaryName above still returns the lowercase form because
// that is what the bundled vendor/ files are called inside this package; only
// the name used to fetch from a release differs.
function getReleaseAssetName(platform = os.type(), arch = os.arch()) {
    return `Proofboard-Career-Agent-${getBinaryName(platform, arch)
        .replace(/^proofboard-/, '')}`;
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
    const releaseFrom = async (url) => {
        const response = await fetchBuffer(url);
        if (response.statusCode !== 200) {
            return null;
        }
        try {
            const parsed = JSON.parse(response.data.toString('utf8'));
            const version = parsed.version || DEFAULT_VERSION;
            const releaseTag = version.startsWith('v') ? version : `v${version}`;
            return {
                version,
                url: parsed.url || `https://proofboard.io/${releaseTag}`,
            };
        } catch (err) {
            return null;
        }
    };

    const primary = await releaseFrom(releasesUrl);
    if (primary) {
        return primary;
    }
    if (releasesUrl === DEFAULT_RELEASES_URL) {
        const github = await releaseFrom(GITHUB_LATEST_RELEASE_URL);
        if (github) {
            return github;
        }
    }
    return {
        version: DEFAULT_VERSION,
        url: `https://github.com/Proofboard-inc/proofboard-cli/releases/latest/download`,
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
    const binaryName = options.binaryName || getBinaryName(options.platform, options.arch);
    const bundledDir = options.bundledDir || path.join(__dirname, 'vendor');
    const bundledBinaryPath = path.join(bundledDir, binaryName);
    const bundledSignaturePath = `${bundledBinaryPath}.sig`;
    if (fs.existsSync(bundledBinaryPath) && fs.existsSync(bundledSignaturePath)) {
        verifyBinarySignature(bundledBinaryPath, bundledSignaturePath, options.publicKey);
        fs.chmodSync(bundledBinaryPath, 0o755);
        return {
            binaryPath: bundledBinaryPath,
            version: options.version || DEFAULT_VERSION,
            binaryName,
        };
    }

    const release = options.version ? {
        version: options.version,
        url: options.releaseBaseUrl || `https://proofboard.io/${options.version.startsWith('v') ? options.version : `v${options.version}`}`,
    } : await getLatestReleaseInfo(options.releasesUrl);
    const version = release.version;
    const cacheDir = options.cacheDir || path.join(os.homedir(), '.proofboard', 'bin', version);
    const binaryPath = path.join(cacheDir, binaryName);
    const signaturePath = `${binaryPath}.sig`;
    // Fetched under the release asset name, cached under the bundled name.
    const assetName = options.assetName || getReleaseAssetName(options.platform, options.arch);
    const downloadUrl = options.downloadUrl || `${release.url.replace(/\/$/, '')}/${assetName}`;
    const signatureUrl = options.signatureUrl || `${downloadUrl}.sig`;
    const releaseTag = version.startsWith('v') ? version : `v${version}`;
    const githubDownloadUrl = `https://github.com/Proofboard-inc/proofboard-cli/releases/download/${releaseTag}/${assetName}`;

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
            if (options.downloadUrl || downloadUrl.startsWith('https://github.com/')) {
                throw err;
            }
            await downloadBinary(githubDownloadUrl, binaryPath);
            await downloadBinary(`${githubDownloadUrl}.sig`, signaturePath);
            verifyBinarySignature(binaryPath, signaturePath, options.publicKey);
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
    DEFAULT_RELEASES_URL,
    GITHUB_LATEST_RELEASE_URL,
    getBinaryName,
    getReleaseAssetName,
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
