#!/usr/bin/env bash
set -euo pipefail

REPO="conglinyizhi/better-edit-tools-mcp"
BIN_NAME="better-edit-tools"
VERSION="${BETTER_EDIT_TOOLS_VERSION:-latest}"

if [ $# -gt 0 ]; then
  VERSION="$1"
fi

# 安装目标：~/.local/share/better-edit-tools/bin/
INSTALL_DIR="${HOME}/.local/share/better-edit-tools/bin"
BIN_PATH="${INSTALL_DIR}/${BIN_NAME}"

detect_os() {
  case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
    linux) echo "linux" ;;
    darwin) echo "darwin" ;;
    *)
      echo "unsupported"
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *)
      echo "unsupported"
      ;;
  esac
}

# 如果已经安装，检查版本
if [ -f "${BIN_PATH}" ]; then
  echo "已安装: ${BIN_PATH}"
  echo "如需更新请先删除: rm ${BIN_PATH}"
  exit 0
fi

OS="$(detect_os)"
ARCH="$(detect_arch)"
if [ "${OS}" = "unsupported" ] || [ "${ARCH}" = "unsupported" ]; then
  echo "不支持的系统或架构: ${OS}/${ARCH}"
  exit 1
fi

echo "⟳ 下载 ${VERSION} 版本..."
mkdir -p "${INSTALL_DIR}"

ARCHIVE="better-edit-tools-${OS}-${ARCH}.tar.gz"
CHECKSUMS_URL="https://github.com/${REPO}/releases/${VERSION}/download/checksums.txt"
URL="https://github.com/${REPO}/releases/${VERSION}/download/${ARCHIVE}"
TMPFILE=$(mktemp)
SHAFILE=$(mktemp)
trap 'rm -f "${TMPFILE}"' EXIT
trap 'rm -f "${SHAFILE}"' EXIT

if command -v curl &>/dev/null; then
  curl -fsSL "${CHECKSUMS_URL}" -o "${SHAFILE}"
  curl -fsSL "${URL}" -o "${TMPFILE}"
elif command -v wget &>/dev/null; then
  wget -q "${CHECKSUMS_URL}" -O "${SHAFILE}"
  wget -q "${URL}" -O "${TMPFILE}"
else
  echo "请安装 curl 或 wget"
  exit 1
fi

EXPECTED="$(grep " ${ARCHIVE}$" "${SHAFILE}" | awk '{print $1}' | head -n 1)"
if command -v sha256sum &>/dev/null; then
  ACTUAL="$(sha256sum "${TMPFILE}" | awk '{print $1}')"
elif command -v shasum &>/dev/null; then
  ACTUAL="$(shasum -a 256 "${TMPFILE}" | awk '{print $1}')"
else
  echo "请安装 sha256sum 或 shasum"
  exit 1
fi
if [ -z "${EXPECTED}" ]; then
  echo "未找到 ${ARCHIVE} 的校验和"
  exit 1
fi
if [ "${EXPECTED}" != "${ACTUAL}" ]; then
  echo "校验失败: ${ARCHIVE}"
  exit 1
fi

echo "⟳ 解压..."
tar xzf "${TMPFILE}" -C "${INSTALL_DIR}"
chmod +x "${BIN_PATH}"

echo "✓ 安装完成: ${BIN_PATH}"
echo "⚠ 这是实验性项目，工具名称和参数可能继续变化。不要把具体工具名写死到 prompt 里。"
echo ""
echo "MCP 配置:"
cat <<JSON
{
  "mcp": {
    "better-edit-tools": {
      "type": "local",
      "command": ["${BIN_PATH}"]
    }
  }
}
JSON
