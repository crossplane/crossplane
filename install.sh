#!/bin/sh

set -eu

XP_CHANNEL=${XP_CHANNEL:-stable}
XP_VERSION=${XP_VERSION:-current}

os=$(uname -s)
arch=$(uname -m)
OS=${OS:-"${os}"}
ARCH=${ARCH:-"${arch}"}
OS_ARCH=""

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

url="https://releases.crossplane.io/${XP_CHANNEL}/${XP_VERSION}/bin/${OS_ARCH}/${BIN}"
if ! curl -sfLo crossplane "${url}"; then
  echo "Failed to download Crossplane CLI. Please make sure version ${XP_VERSION} exists on channel ${XP_CHANNEL}."
  exit 1
fi

chmod +x crossplane

echo "crossplane CLI downloaded successfully! Run the following commands to finish installing it:"
echo 
echo sudo mv crossplane /usr/local/bin
echo crossplane --help
echo
echo "Visit https://crossplane.io to get started. 🚀"
echo "Have a nice day! 👋\n"
