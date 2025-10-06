package wireguard

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// GenerateKeyPair 生成 WireGuard 密钥对
func GenerateKeyPair() (privateKey, publicKey string, err error) {
	// 生成私钥 (32 字节随机数)
	var privKey [32]byte
	if _, err := rand.Read(privKey[:]); err != nil {
		return "", "", fmt.Errorf("生成私钥失败: %w", err)
	}

	// Clamp 私钥 (WireGuard 要求)
	privKey[0] &= 248
	privKey[31] &= 127
	privKey[31] |= 64

	// 生成公钥 (Curve25519)
	var pubKey [32]byte
	curve25519.ScalarBaseMult(&pubKey, &privKey)

	privateKey = base64.StdEncoding.EncodeToString(privKey[:])
	publicKey = base64.StdEncoding.EncodeToString(pubKey[:])

	return privateKey, publicKey, nil
}

// PublicKeyFromPrivate 从私钥计算公钥
func PublicKeyFromPrivate(privateKeyB64 string) (publicKey string, err error) {
	// 解码 base64 私钥
	privKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyB64)
	if err != nil {
		return "", fmt.Errorf("解码私钥失败: %w", err)
	}

	if len(privKeyBytes) != 32 {
		return "", fmt.Errorf("私钥长度错误: 期望32字节，实际%d字节", len(privKeyBytes))
	}

	// 复制到固定大小数组
	var privKey [32]byte
	copy(privKey[:], privKeyBytes)

	// Clamp 私钥 (确保符合 WireGuard 要求)
	privKey[0] &= 248
	privKey[31] &= 127
	privKey[31] |= 64

	// 计算公钥
	var pubKey [32]byte
	curve25519.ScalarBaseMult(&pubKey, &privKey)

	publicKey = base64.StdEncoding.EncodeToString(pubKey[:])
	return publicKey, nil
}
