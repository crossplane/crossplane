#!/bin/sh

set -eu

XP_CHANNEL=${XP_CHANNEL:-stable}
XP_VERSION=${XP_VERSION:-current}

os=$(uname -s)
arch=$(uname -m)
OS=${OS:-"${os}"}
ARCH=${ARCH:-"${arch}"}
OS_ARCH=""
COMPRESSED=${COMPRESSED:-"False"}

BIN=${BIN:-crank}

unsupported_arch() {
  local os=$1
  local arch=$2
  echo "Crossplane does not support $os / $arch at this time."
  exit 1
}

case $OS in
  CYGWIN* | MINGW64* | Windows*)
    if [ $ARCH = "x86_64" ]
    then
      OS_ARCH=windows_amd64
      BIN=crank.exe
    else
      unsupported_arch $OS $ARCH
    fi
    ;;
  Darwin)
    case $ARCH in
      x86_64|amd64)
        OS_ARCH=darwin_amd64
        ;;
      arm64)
        OS_ARCH=darwin_arm64
        ;;
      *)
        unsupported_arch $OS $ARCH
        ;;
    esac
    ;;
  Linux)
    case $ARCH in
      x86_64|amd64)
        OS_ARCH=linux_amd64
        ;;
      arm64|aarch64)
        OS_ARCH=linux_arm64
        ;;
      *)
        unsupported_arch $OS $ARCH
        ;;
    esac
    ;;
  *)
    unsupported_arch $OS $ARCH
    ;;
esac

_compr=`echo $COMPRESSED | tr '[:upper:]' '[:lower:]'`

if [ "${_compr}" = "true" ]; then
    url_dir="bundle"
    url_file="crank.tar.gz"
    url_error="a compressed file for "
else
    url_dir="bin"
    url_file="${BIN}"
    url_error=""
fi

url="https://releases.crossplane.io/${XP_CHANNEL}/${XP_VERSION}/${url_dir}/${OS_ARCH}/${url_file}"

if ! curl -sfL "${url}" -o "${url_file}"; then
    echo "Failed to download Crossplane CLI. Please make sure ${url_error}version ${XP_VERSION} exists on channel ${XP_CHANNEL}."
    exit 1
fi

if [ "${_compr}" = "true" ]; then
    if ! tar xzf "${url_file}"; then
        echo "Failed to unpack the Crossplane CLI compressed file."
        exit 1
    fi
    if ! mv "${BIN}" crossplane; then
        echo "Failed to rename the unpacked Crossplane CLI binary: \"${BIN}\". Make sure it exists inside the compressed file."
        exit 1
    fi
    rm "${BIN}.sha256" "${url_file}"
else
    mv "${url_file}" crossplane
fi

chmod +x crossplane

echo "crossplane CLI downloaded successfully! Run the following commands to finish installing it:"
echo
echo sudo mv crossplane /usr/local/bin
echo crossplane --help
echo
echo "Visit https://crossplane.io to get started. 🚀"
echo "Have a nice day! 👋\n"
