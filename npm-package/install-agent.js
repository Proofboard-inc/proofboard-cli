const { spawnSync } = require('child_process');
const { ensureBinary } = require('./index');

ensureBinary().then(({ binaryPath }) => {
    const result = spawnSync(binaryPath, ['agent', 'enable'], {
        stdio: 'inherit',
        env: process.env,
    });
    if (result.error) {
        throw result.error;
    }
    if (result.status !== 0) {
        throw new Error(`Career Agent registration exited with status ${result.status}`);
    }
}).catch((err) => {
    console.error('Unable to configure Proofboard Career Agent:', err.message);
    process.exit(1);
});
