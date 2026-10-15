#!/bin/bash
# packages a catalog release on the mac. run from repo root, after
# scripts/build-mixxx.sh and then scripts/bundle-mixxx.sh (both in docker, see
# their headers). nothing here runs on the push.
# output:
#   dist/mixxx.tar.gz         the hack. push-catalog unpacks it over
#                             /data/push-hack/hacks/mixxx/, so no settings/ in it
#   dist/mixxx-source.tar.gz  gpl: the mixxx source + patches this build came from
#   release.json              commit it after the github release exists
# then: gh release create v$VERSION dist/mixxx.tar.gz dist/mixxx-source.tar.gz
set -euo pipefail
REPO=itsrave/push-mixxx-mode
V=$(python3 -c 'import json; print(json.load(open("hack.json"))["version"])')

# a bundle older than the patched .deb would ship the old mixxx
[ out/mixxx/bin/mixxx -nt "$(ls out/debs/*.deb)" ] || { echo "out/mixxx is older than out/debs: run bundle-mixxx.sh" >&2; exit 1; }

make build
rm -rf dist && mkdir -p dist/mixxx
cp -R out/mixxx/. dist/mixxx/ # bin lib plugins share mixxx.sh
cp -R hack.json mixxx-bridge mapping skin README.md LICENSE dist/mixxx/
rm dist/mixxx/mapping/check.js
docker run --rm -v mixxx-build:/build -v "$PWD/dist":/out alpine sh -c \
    'cp /build/mixxx-*/LICENSE /out/mixxx/LICENSE.mixxx && mkdir /out/src && cp /build/mixxx_*.orig.tar.gz /build/mixxx_*.diff.gz /build/mixxx_*.dsc /out/src/'
[ ! -e dist/mixxx/settings ] # an update would overwrite the user's library
COPYFILE_DISABLE=1 tar -czf dist/mixxx.tar.gz -C dist mixxx

cp -R patches scripts dist/src/
COPYFILE_DISABLE=1 tar -czf dist/mixxx-source.tar.gz -C dist src

printf '{\n  "version": "%s",\n  "download_url": "https://github.com/%s/releases/download/v%s/mixxx.tar.gz",\n  "released_at": "%s"\n}\n' \
    "$V" "$REPO" "$V" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > release.json
ls -lh dist/*.tar.gz
