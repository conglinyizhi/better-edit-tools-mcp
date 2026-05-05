#!/usr/bin/env bash
set -euo pipefail

REPO="conglinyizhi/better-edit-tools-mcp"
ARCHIVE="better-edit-tools-x86_64-unknown-linux-musl.tar.gz"
BIN_NAME="better-edit-tools"

# 安装目标：~/.local/share/better-edit-tools/bin/
INSTALL_DIR="${HOME}/.local/share/better-edit-tools/bin"
BIN_PATH="${INSTALL_DIR}/${BIN_NAME}"

# 如果已经安装，检查版本
if [ -f "${BIN_PATH}" ]; then
  echo "已安装: ${BIN_PATH}"
  echo "如需更新请先删除: rm ${BIN_PATH}"
  exit 0
fi

echo "⟳ 下载 latest 版本..."
mkdir -p "${INSTALL_DIR}"

URL="https://github.com/${REPO}/releases/latest/download/${ARCHIVE}"
TMPFILE=$(mktemp)
trap 'rm -f "${TMPFILE}"' EXIT

if command -v curl &>/dev/null; then
  curl -fsSL "${URL}" -o "${TMPFILE}"
elif command -v wget &>/dev/null; then
  wget -q "${URL}" -O "${TMPFILE}"
else
  echo "请安装 curl 或 wget"
  exit 1
fi

echo "⟳ 解压..."
tar xzf "${TMPFILE}" -C "${INSTALL_DIR}"
chmod +x "${BIN_PATH}"

echo "✓ 安装完成: ${BIN_PATH}"
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
