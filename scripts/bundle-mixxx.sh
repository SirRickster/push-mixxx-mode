#!/bin/bash
# builds the mixxx bundle for push 3 (glibc 2.35 = ubuntu 22.04). run from repo root:
#   docker run --rm --platform linux/amd64 -v "$PWD":/work ubuntu:22.04 /work/scripts/bundle-mixxx.sh
# output: out/mixxx/ (bin, lib, plugins, share, mixxx.sh). push has no /lib64, so
# mixxx.sh starts mixxx through push's own /lib/ld-linux-x86-64.so.2.
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq software-properties-common >/dev/null
add-apt-repository -y ppa:mixxx/mixxx >/dev/null
apt-get update -qq
apt-get install -y -qq --no-install-recommends mixxx libgl1-mesa-dri libglx-mesa0 libegl-mesa0 qt6-qpa-plugins libqt6sql6-sqlite qt6-image-formats-plugins libqt6svg6 >/dev/null
# patched build from scripts/build-mixxx.sh, if there is one
if ls /work/out/debs/*.deb >/dev/null 2>&1; then apt-get install -y -qq /work/out/debs/*.deb >/dev/null; fi

B=/work/out/mixxx; rm -rf $B; mkdir -p $B/bin $B/lib $B/plugins $B/share
Q=/usr/lib/x86_64-linux-gnu/qt6/plugins
cp /usr/bin/mixxx $B/bin/
for d in platforms xcbglintegrations sqldrivers imageformats iconengines tls; do [ -d $Q/$d ] && cp -r $Q/$d $B/plugins/; done
mkdir -p $B/lib/dri
# iris = this push's gpu (tiger lake). push itself only ships crocus.
cp /usr/lib/x86_64-linux-gnu/dri/{iris,swrast,kms_swrast}_dri.so $B/lib/dri/
cp -r /usr/share/mixxx $B/share/
[ -d /usr/share/glvnd ] && cp -r /usr/share/glvnd $B/share/

# every non-glibc lib that any bundled ELF needs. glibc 2.35 on device = jammy's, so use the device's.
GLIBC='^(libc|libm|libpthread|libdl|librt|libresolv|libutil|libanl|ld-linux-x86-64)\.so'
find $B/bin $B/plugins $B/lib/dri -type f \( -name '*.so*' -o -perm -u+x \) | while read f; do ldd "$f" 2>/dev/null; done \
  | awk '/=> \//{print $3}' | sort -u | while read l; do
    n=$(basename $l); [[ $n =~ $GLIBC ]] || cp -nL $l $B/lib/
  done
# mesa glvnd vendor libs are dlopen'ed, ldd can't see them
cp -nL /usr/lib/x86_64-linux-gnu/libGLX_mesa.so.0 /usr/lib/x86_64-linux-gnu/libEGL_mesa.so.0 $B/lib/ 2>/dev/null || true
for f in $B/lib/libGLX_mesa.so.0 $B/lib/libEGL_mesa.so.0; do ldd $f | awk '/=> \//{print $3}'; done | sort -u | while read l; do n=$(basename $l); [[ $n =~ $GLIBC ]] || cp -nL $l $B/lib/; done
ldd $B/bin/mixxx | grep 'not found' || echo "no missing libs"

cat > $B/mixxx.sh <<'SH'
#!/bin/sh
# launch bundled mixxx on push's hidden X screen.
D=$(cd "$(dirname "$0")" && pwd)
export LD_LIBRARY_PATH="$D/lib" QT_PLUGIN_PATH="$D/plugins" LIBGL_DRIVERS_PATH="$D/lib/dri"
export __EGL_VENDOR_LIBRARY_DIRS="$D/share/glvnd/egl_vendor.d" DISPLAY="${DISPLAY:-:0}"
exec /lib/ld-linux-x86-64.so.2 "$D/bin/mixxx" --resource-path "$D/share/mixxx/" --settings-path "${MIXXX_SETTINGS:-/data/push-hack/hacks/mixxx/settings}/" "$@"
SH
chmod +x $B/mixxx.sh
dpkg -s mixxx | grep ^Version; du -sh $B $B/lib $B/lib/dri $B/share/mixxx
