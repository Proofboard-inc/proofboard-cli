const assert = require('assert');
const crypto = require('crypto');
const fs = require('fs');
const os = require('os');
const path = require('path');
const {
    DEFAULT_VERSION,
    ensureBinary,
    getBinaryName,
    getLatestReleaseInfo,
    verifyBinarySignature,
} = require('./index');

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
    const fallback = await getLatestReleaseInfo('http://example.com/latest.json');
    assert.strictEqual(fallback.version, DEFAULT_VERSION);

    const invalidCache = fs.mkdtempSync(path.join(os.tmpdir(), 'proofboard-npm-cache-test-'));
    try {
        const binaryName = 'proofboard-linux-amd64';
        const binaryPath = path.join(invalidCache, binaryName);
        fs.writeFileSync(binaryPath, Buffer.from('tampered cache'));
        fs.writeFileSync(`${binaryPath}.sig`, Buffer.from('invalid signature'));
        await assert.rejects(
            ensureBinary({
                version: DEFAULT_VERSION,
                binaryName,
                cacheDir: invalidCache,
                downloadUrl: 'http://example.com/proofboard',
            }),
            /Failed to download binary/,
        );
        assert.strictEqual(fs.existsSync(binaryPath), false);
        assert.strictEqual(fs.existsSync(`${binaryPath}.sig`), false);
    } finally {
        fs.rmSync(invalidCache, { recursive: true, force: true });
    }
})().catch((err) => {
    console.error(err);
    process.exitCode = 1;
});
