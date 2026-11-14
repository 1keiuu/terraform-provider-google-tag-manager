#!/usr/bin/env python3
"""Audit every vendored dependency and verify the artifacts selected by workflows."""

import hashlib
import json
from pathlib import Path
import re
import subprocess
import urllib.request


ROOT = Path(__file__).resolve().parents[1]
ACTIONS = ROOT / '.github' / 'actions'
REPOSITORY = '1keiuu/terraform-provider-google-tag-manager'


def main():
    sources = json.loads((ACTIONS / 'sources.json').read_text())
    for name in sources:
        directory = ACTIONS / name
        files = json.loads((directory / 'bundle-files.json').read_text())
        for relative, expected in files.items():
            actual = hashlib.sha256((directory / relative).read_bytes()).hexdigest()
            if actual != expected:
                raise RuntimeError(f'{name}: artifact checksum mismatch: {relative}')
        result = subprocess.run(
            ['npm', 'audit', '--ignore-scripts', '--json'], cwd=directory,
            text=True, capture_output=True,
        )
        report = json.loads(result.stdout)
        if result.returncode or report['metadata']['vulnerabilities']['total']:
            raise RuntimeError(f'{name}: dependency audit failed: {result.stdout}')
        print(f'{name}: all runtime and build dependencies passed', flush=True)

    checkout_commits = set()
    for workflow in (ROOT / '.github' / 'workflows').glob('*.yml'):
        for reference in re.findall(r'^\s*(?:-\s*)?uses:\s*([^\s#]+)', workflow.read_text(), re.MULTILINE):
            local = re.fullmatch(r'\./\.github/actions/([a-z-]+)(?:/[a-z-]+)?', reference)
            if local and (local[1] in sources or local[1] == 'node-runtime'):
                continue
            checkout = re.fullmatch(re.escape(REPOSITORY) + r'/\.github/actions/checkout@([0-9a-f]{40})', reference)
            if not checkout:
                raise RuntimeError(f'{workflow.name}: unaudited Action reference: {reference}')
            checkout_commits.add(checkout[1])
    if len(checkout_commits) != 1:
        raise RuntimeError('All workflows must use one immutable checkout commit')
    commit, = checkout_commits
    # Checkout runs before local files exist. Verify its immutable remote copy
    # matches the lockfile and artifact manifest audited in this checkout.
    for filename in ('package.json', 'package-lock.json', 'bundle-files.json'):
        url = f'https://raw.githubusercontent.com/{REPOSITORY}/{commit}/.github/actions/checkout/{filename}'
        with urllib.request.urlopen(url, timeout=30) as response:
            remote = response.read()
        if remote != (ACTIONS / 'checkout' / filename).read_bytes():
            raise RuntimeError(f'Pinned checkout differs from audited files: {filename}')
    print(f'{len(sources)} Actions and all workflow references verified')


if __name__ == '__main__':
    main()
