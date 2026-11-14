const fs = require('node:fs');
const path = require('node:path');

fs.appendFileSync(process.env.GITHUB_PATH, `${path.dirname(process.execPath)}\n`);
