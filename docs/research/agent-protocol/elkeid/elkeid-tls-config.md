# Elkeid 上游实际 TLS 配置代码（用于查证）

本文摘录 Elkeid 固定提交 `c906c512` 中客户端和服务端构造 TLS 凭据的实际代码，供 [AIDR Agent 协议](../endpoint-platform-v2.md) 第 1.1 节查证。AIDR 只核对参考行为，不照搬源码。

## Agent 端：setDialOptions

`go:embed` 只提供文件字节。在 Elkeid 源码里，`agent/transport/connection/product.go` 是产品版初始化文件，在包启动时取得构建时嵌入的三个 PEM，然后调用：

~~~go
setDialOptions(CaCert, ClientKey, ClientCert, "elkeid.com")
~~~

下面是实际的 `setDialOptions`。它创建 `tls.Config`，把客户端证书和私钥放入 `Certificates`，再把 TLS 凭据写入 gRPC 的 `dialOptions`。 [产品版入口][elkeid-product] · [TLS 配置源码][elkeid-dial]

~~~go
func setDialOptions(ca, privkey, cert []byte, svrName string) {
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(ca)
	keyPair, _ := tls.X509KeyPair(cert, privkey)
	dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		Certificates:       []tls.Certificate{keyPair},
		ClientAuth:         tls.RequireAndVerifyClientCert,
		RootCAs:            certPool,
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			certs := make([]*x509.Certificate, len(rawCerts))
			for i, asn1Data := range rawCerts {
				cert, err := x509.ParseCertificate(asn1Data)
				if err != nil {
					return errors.New("tls: failed to parse certificate from server: " + err.Error())
				}
				certs[i] = cert
			}
			opts := x509.VerifyOptions{
				Roots:         certPool,
				DNSName:       svrName,
				Intermediates: x509.NewCertPool(),
			}
			for _, cert := range certs[1:] {
				opts.Intermediates.AddCert(cert)
			}
			_, err := certs[0].Verify(opts)
			return err
		},
	})), grpc.WithStatsHandler(&DefaultStatsHandler), grpc.WithBlock(), grpc.WithReturnConnectionError(), grpc.FailOnNonTempDialError(true))
}
~~~

固定版设置 `InsecureSkipVerify: true` 以跳过默认校验，实际由 `VerifyPeerCertificate` 回调用指定 CA 校验证书链、有效期和服务端名称。回调返回错误时，握手失败。

## 服务端：grpctrans.Run

上游进程启动入口 `grpctrans.Run()` 从部署配置加载证书、私钥和客户端信任 CA。它把下面的 TLS 凭据通过 `grpc.Creds(ct)` 交给 gRPC。 `RequireAndVerifyClientCert` 要求 Agent 出示证书并由库验证。 [服务端 TLS 与启动入口][elkeid-tls]

~~~go
return credentials.NewTLS(&tls.Config{
    Certificates: []tls.Certificate{cert},
    ClientAuth:   tls.RequireAndVerifyClientCert,
    ClientCAs:    certPool,
})
~~~

[elkeid-product]: https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/agent/transport/connection/product.go
[elkeid-dial]: https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/agent/transport/connection/connection.go
[elkeid-tls]: https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/server/agent_center/grpctrans/grpc_server.go
