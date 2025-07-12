#!/bin/bash

echo "🔐 生成MEEA-VIOFO许可证系统RSA密钥对..."

# 创建密钥目录
mkdir -p keys

# 生成通信密钥对 (客户端加密 -> 服务端解密)
echo "📡 生成通信密钥对..."
openssl genrsa -out keys/comm_private_key.pem 2048
openssl rsa -in keys/comm_private_key.pem -pubout -out keys/comm_public_key.pem

# 生成签名密钥对 (服务端签名 -> 客户端验证)
echo "✍️ 生成签名密钥对..."
openssl genrsa -out keys/sign_private_key.pem 2048
openssl rsa -in keys/sign_private_key.pem -pubout -out keys/sign_public_key.pem

# 设置密钥文件权限
chmod 600 keys/*_private_key.pem
chmod 644 keys/*_public_key.pem

echo "✅ 密钥生成完成!"
echo ""
echo "📋 客户端需要的公钥:"
echo "通信公钥 (用于加密发送数据):"
cat keys/comm_public_key.pem
echo ""
echo "签名公钥 (用于验证证书签名):"
cat keys/sign_public_key.pem
