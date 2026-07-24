const { spawnSync } = require('child_process');
const { ensureBinary } = require('./index');

ensureBinary().then(({ binaryPath }) => {
    const result = spawnSync(binaryPath, ['agent', 'disable'], {
        stdio: 'inherit',
        env: process.env,
    });
    if (result.error) {
        throw result.error;
    }
    if (result.status !== 0) {
        throw new Error(`Career Agent removal exited with status ${result.status}`);
    }
}).catch((err) => {
    console.error('Unable to remove Proofboard Career Agent registration:', err.message);
    process.exit(1);
});
