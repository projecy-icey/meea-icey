#!/bin/bash

# 安装 Homebrew（如果未安装）
if ! command -v brew &>/dev/null; then
  echo "Homebrew 未安装，正在安装 Homebrew..."
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
fi

# 安装 git-lfs
echo "正在安装 git-lfs..."
brew install git-lfs

# 初始化 git-lfs
echo "正在初始化 git-lfs..."
git lfs install

# 检查 git-lfs 版本
echo "git-lfs 版本："
git lfs version

echo "git-lfs 安装并初始化完成！" 