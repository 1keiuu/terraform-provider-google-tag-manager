#!/usr/bin/env python3
"""Check every selected Go module, including graph-only dependencies, against OSV."""

import json
import subprocess
import sys
import time
import urllib.error
import urllib.request


def modules():
    output = subprocess.check_output(['go', 'list', '-m', '-json', 'all'], text=True)
    decoder = json.JSONDecoder()
    while output.strip():
        output = output.lstrip()
        module, end = decoder.raw_decode(output)
        output = output[end:]
        if module.get('Main'):
            continue
        module = module.get('Replace', module)
        if not module.get('Version'):
            raise RuntimeError(f"Cannot audit an unversioned module: {module['Path']}")
        yield module


def query_osv(queries):
    request = urllib.request.Request(
        'https://api.osv.dev/v1/querybatch',
        data=json.dumps({'queries': queries}).encode(),
        headers={'Content-Type': 'application/json'},
    )
    for attempt in range(3):
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                results = json.load(response)['results']
            if len(results) != len(queries):
                raise RuntimeError('OSV returned an incomplete result set')
            return results
        except (urllib.error.URLError, TimeoutError):
            if attempt == 2:
                raise
            time.sleep(2 ** attempt)


def main():
    queries = [{'package': {'name': m['Path'], 'ecosystem': 'Go'}, 'version': m['Version']}
               for m in modules()]
    findings = []
    for offset in range(0, len(queries), 1000):
        batch = queries[offset:offset + 1000]
        for query, result in zip(batch, query_osv(batch)):
            for vulnerability in result.get('vulns', []):
                findings.append(f"{query['package']['name']} {query['version']}: {vulnerability['id']}")
    if findings:
        print('\n'.join(findings), file=sys.stderr)
        return 1
    print(f'OSV: no known vulnerabilities in {len(queries)} selected Go modules')
    return 0


if __name__ == '__main__':
    sys.exit(main())
