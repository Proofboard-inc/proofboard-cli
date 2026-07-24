#!/usr/bin/env node

const { run } = require('../index');

run(process.argv.slice(2)).then((code) => {
    process.exit(code);
}).catch((err) => {
    console.error('Error running Proofboard Career Agent:', err.message);
    process.exit(1);
});
