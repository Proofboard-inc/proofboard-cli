const assert = require('assert');
const crypto = require('crypto');
const fs = require('fs');
const os = require('os');
const path = require('path');
const {
    DEFAULT_RELEASES_URL,
    DEFAULT_VERSION,
    GITHUB_LATEST_RELEASE_URL,
    ensureBinary,
    getBinaryName,
    verifyBinarySignature,
} = require('./index');

assert.strictEqual(DEFAULT_RELEASES_URL, 'https://proofboard.io/latest.json');
assert.strictEqual(
    GITHUB_LATEST_RELEASE_URL,
    'https://github.com/Proofboard-inc/proofboard-cli/releases/latest/download/latest.json',
);
assert.strictEqual(getBinaryName('Linux', 'x64'), 'proofboard-linux-amd64');
assert.strictEqual(getBinaryName('Darwin', 'x64'), 'proofboard-darwin-amd64');
assert.strictEqual(getBinaryName('Darwin', 'arm64'), 'proofboard-darwin-arm64');
assert.strictEqual(getBinaryName('Windows_NT', 'x64'), 'proofboard-windows-amd64.exe');
assert.throws(() => getBinaryName('Linux', 'arm64'), /Unsupported platform/);

const temporaryDirectory = fs.mkdtempSync(path.join(os.tmpdir(), 'proofboard-npm-test-'));
try {
    const binaryPath = path.join(temporaryDirectory, 'proofboard');
    const signaturePath = `${binaryPath}.sig`;
    const payload = Buffer.from('signed Career Agent test binary');
    const { publicKey, privateKey } = crypto.generateKeyPairSync('ec', { namedCurve: 'prime256v1' });
    fs.writeFileSync(binaryPath, payload);
    fs.writeFileSync(signaturePath, crypto.sign('sha256', payload, privateKey));
    assert.doesNotThrow(() => verifyBinarySignature(binaryPath, signaturePath, publicKey));
    fs.writeFileSync(binaryPath, Buffer.from('tampered binary'));
    assert.throws(
        () => verifyBinarySignature(binaryPath, signaturePath, publicKey),
        /signature verification failed/,
    );
} finally {
    fs.rmSync(temporaryDirectory, { recursive: true, force: true });
}

(async () => {
    const bundledDirectory = fs.mkdtempSync(path.join(os.tmpdir(), 'proofboard-npm-bundle-test-'));
    try {
        const binaryName = 'proofboard-linux-amd64';
        const binaryPath = path.join(bundledDirectory, binaryName);
        const payload = Buffer.from('bundled signed Career Agent');
        const { publicKey, privateKey } = crypto.generateKeyPairSync('ec', { namedCurve: 'prime256v1' });
        fs.writeFileSync(binaryPath, payload);
        fs.writeFileSync(`${binaryPath}.sig`, crypto.sign('sha256', payload, privateKey));
        const bundled = await ensureBinary({
            version: DEFAULT_VERSION,
            binaryName,
            bundledDir: bundledDirectory,
            publicKey,
        });
        assert.strictEqual(bundled.binaryPath, binaryPath);
        assert.strictEqual(bundled.version, DEFAULT_VERSION);
        assert.strictEqual(fs.statSync(binaryPath).mode & 0o777, 0o755);
    } finally {
        fs.rmSync(bundledDirectory, { recursive: true, force: true });
    }
})().catch((err) => {
    console.error(err);
    process.exitCode = 1;
});
