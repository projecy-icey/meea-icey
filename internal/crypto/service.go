package crypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io/ioutil"
)

type Service struct {
	commPrivateKey *rsa.PrivateKey
	signPrivateKey *rsa.PrivateKey
}

func NewService(commKeyPath, signKeyPath string) (*Service, error) {
	commKey, err := loadPrivateKey(commKeyPath)
	if err != nil {
		return nil, err
	}

	signKey, err := loadPrivateKey(signKeyPath)
	if err != nil {
		return nil, err
	}

	return &Service{
		commPrivateKey: commKey,
		signPrivateKey: signKey,
	}, nil
}

// 解密客户端数据
func (s *Service) DecryptClientData(encryptedData string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", err
	}

	decrypted, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, s.commPrivateKey, data, nil)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}

// 签名证书数据
func (s *Service) SignCertificate(certData interface{}) (string, error) {
	dataBytes, err := json.Marshal(certData)
	if err != nil {
		return "", err
	}

	hashed := sha256.Sum256(dataBytes)
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.signPrivateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// 加载私钥
func loadPrivateKey(keyPath string) (*rsa.PrivateKey, error) {
	keyData, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// 尝试PKCS1格式
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an RSA private key")
	}

	return rsaKey, nil
}
