#!/usr/bin/env python3
"""Rebuild pinned Action sources with audited, locked dependencies."""

import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import urllib.request


ROOT = Path(__file__).resolve().parents[1]
ACTIONS = ROOT / '.github' / 'actions'


def run(command, cwd):
    subprocess.run(command, cwd=cwd, check=True)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def licenses(source):
    lock = json.loads((source / 'package-lock.json').read_text())
    sections = []
    for relative, package in sorted(lock['packages'].items()):
        if not relative or package.get('dev'):
            continue
        directory = source / relative
        if not directory.is_dir():
            continue
        texts = [p.read_text(errors='replace') for p in sorted(directory.iterdir())
                 if p.is_file() and p.name.lower().startswith(('license', 'licence', 'notice', 'copying'))]
        sections.append('\n'.join([
            f"{relative} {package['version']}",
            f"License: {package.get('license', 'See upstream package')}",
            *texts,
        ]))
    return '\n\n'.join(sections) + '\n'


def build(name, specification, check):
    destination = ACTIONS / name
    with tempfile.TemporaryDirectory(prefix=f'gtm-action-{name}-') as temporary:
        work = Path(temporary)
        archive = work / 'source.tar.gz'
        url = f"https://codeload.github.com/{specification['repository']}/tar.gz/{specification['commit']}"
        urllib.request.urlretrieve(url, archive)
        if digest(archive) != specification['archive_sha256']:
            raise RuntimeError(f'{name}: source archive checksum mismatch')
        extracted = work / 'source'
        extracted.mkdir()
        with tarfile.open(archive) as tar:
            tar.extractall(extracted, filter='data')
        source, = extracted.iterdir()
        static_files = {relative: (source / relative).read_bytes()
                        for relative in specification.get('static_files', [])}
        # Never carry upstream bundles or source maps into patched artifacts.
        for relative in specification['generated_directories']:
            directory = source / relative
            if directory.exists():
                shutil.rmtree(directory)
        for filename in ('package.json', 'package-lock.json'):
            shutil.copyfile(destination / filename, source / filename)
        # Ignore all dependency lifecycle hooks. Only the reviewed build command runs.
        run(['npm', 'ci', '--ignore-scripts', '--no-audit', '--no-fund'], source)
        run(['npm', 'audit', '--ignore-scripts', '--audit-level=low'], source)
        if 'typescript_entrypoints' in specification:
            # Use TypeScript's parser for upstream JSON-with-comments configs.
            # Typecheck the executable Action and its imports, excluding upstream
            # documentation generators and test harnesses from this runtime build.
            script = """
const fs = require('node:fs');
const ts = require('typescript');
const result = ts.readConfigFile('tsconfig.json', ts.sys.readFile);
if (result.error) throw new Error(ts.flattenDiagnosticMessageText(result.error.messageText, '\\n'));
result.config.include = JSON.parse(process.argv[1]);
fs.writeFileSync('tsconfig.json', JSON.stringify(result.config, null, 2) + '\\n');
"""
            run(['node', '--input-type=commonjs', '-e', script,
                 json.dumps(specification['typescript_entrypoints'])], source)
        run(['npm', 'run', 'build'], source)
        for relative, content in static_files.items():
            target = source / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(content)
        output = work / 'output'
        output.mkdir()
        for relative in specification['outputs']:
            target = output / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            item = source / relative
            if item.is_dir():
                shutil.copytree(item, target)
            else:
                shutil.copyfile(item, target)
        license_files = [p for p in source.iterdir()
                         if p.is_file() and p.name.lower().startswith(('license', 'licence', 'notice'))]
        for item in license_files:
            shutil.copyfile(item, output / item.name)
        (output / 'THIRD_PARTY_LICENSES.txt').write_text(licenses(source))
        hashes = {str(p.relative_to(output)): digest(p)
                  for p in sorted(output.rglob('*')) if p.is_file()}
        manifest = destination / 'bundle-files.json'
        if check:
            expected = json.loads(manifest.read_text())
            if expected != hashes:
                changed = sorted(k for k in expected.keys() | hashes.keys() if expected.get(k) != hashes.get(k))
                raise RuntimeError(f'{name}: rebuild differs: {changed}')
            for relative, checksum in expected.items():
                if digest(destination / relative) != checksum:
                    raise RuntimeError(f'{name}: checked-in artifact differs: {relative}')
        else:
            if manifest.exists():
                for relative in json.loads(manifest.read_text()):
                    (destination / relative).unlink(missing_ok=True)
            shutil.copytree(output, destination, dirs_exist_ok=True)
            manifest.write_text(json.dumps(hashes, indent=2, sort_keys=True) + '\n')
        print(f'{name}: {len(hashes)} artifacts verified', flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true', help='Verify reproducible builds and checked-in files')
    parser.add_argument('actions', nargs='*', help='Action directory names; defaults to all')
    args = parser.parse_args()
    sources = json.loads((ACTIONS / 'sources.json').read_text())
    for name in args.actions or sources:
        build(name, sources[name], args.check)


if __name__ == '__main__':
    main()
