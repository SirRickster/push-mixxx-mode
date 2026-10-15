#!/bin/bash
# rebuilds the mixxx PPA package (jammy, same flags as the PPA build) with
# patches/*.patch applied. output: out/debs/*.deb, picked up by bundle-mixxx.sh.
#   docker run --rm --platform linux/amd64 -v "$PWD":/work -v mixxx-build:/build \
#     -e PARALLEL=2 ubuntu:22.04 /work/scripts/build-mixxx.sh
# the mixxx-build volume keeps objects: rerun to resume, PARALLEL up when docker has ram.
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive DEBEMAIL=push-hack@localhost DEBFULLNAME=push-hack
apt-get update -qq
apt-get install -y -qq software-properties-common dpkg-dev devscripts >/dev/null
add-apt-repository -y -s ppa:mixxx/mixxx >/dev/null
apt-get update -qq
apt-get build-dep -y -qq mixxx >/dev/null
mkdir -p /build && cd /build
if ! ls -d mixxx-*/ >/dev/null 2>&1; then
    apt-get source -qq mixxx
    cd mixxx-*/
    for p in /work/patches/*.patch; do echo "applying $p"; patch -p1 < "$p"; done
    dch --local +pushhack "push-hack patches: $(cd /work/patches && echo *.patch)"
    # no engine prime (denon) export: its djinterop dep OOMs cc1plus under qemu,
    # and nobody exports to engine dj from a push. no tests: nocheck skips running
    # them, not building them.
    sed -i 's/-DKEYFINDER=OFF/-DKEYFINDER=OFF -DENGINEPRIME=OFF -DBUILD_TESTING=OFF/' debian/rules
    grep -q ENGINEPRIME=OFF debian/rules
    cd ..
fi
cd mixxx-*/
# each cc1plus under qemu takes ~1-1.5 GB: 6 jobs OOM'd an 8 GB docker vm.
# -nc: keep objects from an earlier, interrupted run.
# -lto: ubuntu turns lto on for every package; the final lto1 link OOMs under
# qemu. mixxx itself ships with lto off (CMakeLists.txt, IPO section).
DEB_BUILD_MAINT_OPTIONS=optimize=-lto DEB_BUILD_OPTIONS="nocheck parallel=${PARALLEL:-2}" dpkg-buildpackage -b -uc -us -nc
rm -rf /work/out/debs && mkdir -p /work/out/debs
cp ../*.deb /work/out/debs/
ls -l /work/out/debs
