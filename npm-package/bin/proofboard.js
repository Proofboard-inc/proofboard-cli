#!/usr/bin/env node

const { run } = require('../index');

run(process.argv.slice(2)).then((code) => {
    process.exit(code);
}).catch((err) => {
    console.error('Error running Proofboard CLI via npx:', err.message);
    process.exit(1);
});
