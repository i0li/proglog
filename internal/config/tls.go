package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

func SetupTLSConfig(cfg TLSConfig) (*tls.Config, error) {
	var err error
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS13}
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		//　自身を証明する証明証のリスト
		tlsConfig.Certificates = make([]tls.Certificate, 1)
		tlsConfig.Certificates[0], err = tls.LoadX509KeyPair(
			cfg.CertFile, // 公開鍵証明書(公開鍵も含まれる)
			cfg.KeyFile,  // 秘密鍵
		)
		if err != nil {
			return nil, err
		}
	}
	if cfg.CAFile != "" {
		b, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, err
		}
		// CAを証明するための証明書の集合
		ca := x509.NewCertPool()
		ok := ca.AppendCertsFromPEM([]byte(b))
		if !ok {
			return nil, fmt.Errorf(
				"failed to parse root certificate: %q",
				cfg.CAFile,
			)
		}
		if cfg.Server {
			// クライアントの証明証が信頼できるCAから発行されたのかをチェックするためのもの
			// 具体的にはここで指定したCAから発行されたものかチェックする
			tlsConfig.ClientCAs = ca
			// クライアントに証明証の提出を強制する
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		} else {
			// 証明証が信頼できるCAから発行されたものはチェックするためのもの
			tlsConfig.RootCAs = ca
		}
		tlsConfig.ServerName = cfg.ServerAddress
	}
	return tlsConfig, nil
}

type TLSConfig struct {
	CertFile      string
	KeyFile       string
	CAFile        string
	ServerAddress string
	Server        bool
}
