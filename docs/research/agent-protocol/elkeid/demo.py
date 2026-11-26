"""Elkeid Agent 双向 TLS 认证过程本地 demo。

用 cryptography 临时生成 CA、服务端与客户端证书，启动本地双向 TLS 服务，
复现证书链/DNS 校验、客户端证书校验和首包 agent_id 登记。
不启动真实 gRPC、AgentCenter 或 Kafka。
"""

import datetime
import json
import socket
import ssl
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.x509.oid import ExtendedKeyUsageOID, NameOID

NOW = datetime.datetime.now(datetime.timezone.utc)


def make_ca(name):
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    subject = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, name)])
    cert = (x509.CertificateBuilder()
            .subject_name(subject).issuer_name(subject)
            .public_key(key.public_key())
            .serial_number(x509.random_serial_number())
            .not_valid_before(NOW - datetime.timedelta(days=1))
            .not_valid_after(NOW + datetime.timedelta(days=1))
            .add_extension(x509.BasicConstraints(ca=True, path_length=None), critical=True)
            .add_extension(x509.KeyUsage(
                digital_signature=True, content_commitment=False, key_encipherment=False,
                data_encipherment=False, key_agreement=False, key_cert_sign=True,
                crl_sign=True, encipher_only=False, decipher_only=False), critical=True)
            .add_extension(x509.SubjectKeyIdentifier.from_public_key(key.public_key()),
                           critical=False)
            .sign(key, hashes.SHA256()))
    return key, cert


def issue(ca_key, ca_cert, common_name, dns=None, client=False):
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    subject = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, common_name)])
    builder = (x509.CertificateBuilder()
               .subject_name(subject).issuer_name(ca_cert.subject)
               .public_key(key.public_key())
               .serial_number(x509.random_serial_number())
               .not_valid_before(NOW - datetime.timedelta(days=1))
               .not_valid_after(NOW + datetime.timedelta(days=1)))
    builder = builder.add_extension(x509.KeyUsage(
        digital_signature=True, content_commitment=False, key_encipherment=True,
        data_encipherment=False, key_agreement=False, key_cert_sign=False,
        crl_sign=False, encipher_only=False, decipher_only=False), critical=True)
    builder = builder.add_extension(
        x509.AuthorityKeyIdentifier.from_issuer_public_key(ca_key.public_key()), critical=False)
    builder = builder.add_extension(
        x509.SubjectKeyIdentifier.from_public_key(key.public_key()), critical=False)
    if dns:
        builder = builder.add_extension(
            x509.SubjectAlternativeName([x509.DNSName(dns)]), critical=False)
    usage = ExtendedKeyUsageOID.CLIENT_AUTH if client else ExtendedKeyUsageOID.SERVER_AUTH
    builder = builder.add_extension(x509.ExtendedKeyUsage([usage]), critical=False)
    return key, builder.sign(ca_key, hashes.SHA256())


def to_pem(cert):
    return cert.public_bytes(serialization.Encoding.PEM)


def to_key_pem(key):
    return key.private_bytes(serialization.Encoding.PEM,
                             serialization.PrivateFormat.TraditionalOpenSSL,
                             serialization.NoEncryption())


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass

    def do_GET(self):
        agent_id = self.headers.get("X-Agent-ID", "")
        body = json.dumps({"registered": agent_id, "path": "/grpc.Transfer/Transfer"}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def exchange(ctx, port, hostname, agent_id):
    raw = socket.create_connection(("127.0.0.1", port), timeout=5)
    with ctx.wrap_socket(raw, server_hostname=hostname) as tls:
        request = (f"GET /grpc.Transfer/Transfer HTTP/1.1\r\n"
                   f"Host: {hostname}\r\nX-Agent-ID: {agent_id}\r\n"
                   "Connection: close\r\n\r\n").encode()
        tls.sendall(request)
        chunks = []
        while True:
            part = tls.recv(4096)
            if not part:
                break
            chunks.append(part)
    text = b"".join(chunks).decode(errors="replace")
    return json.loads(text.split("\r\n\r\n", 1)[1])


def main():
    tmp = tempfile.TemporaryDirectory()
    try:
        ca_key, ca_cert = make_ca("Elkeid Demo CA")
        server_key, server_cert = issue(ca_key, ca_cert, "elkeid.com", dns="elkeid.com")
        client_key, client_cert = issue(ca_key, ca_cert, "demo-agent", client=True)
        rogue_ca_key, rogue_ca_cert = make_ca("Rogue CA")
        rogue_key, rogue_cert = issue(rogue_ca_key, rogue_ca_cert, "rogue-agent", client=True)

        paths = {}
        for name, value in [("ca", to_pem(ca_cert)), ("server_cert", to_pem(server_cert)),
                            ("server_key", to_key_pem(server_key)),
                            ("client_cert", to_pem(client_cert)),
                            ("client_key", to_key_pem(client_key)),
                            ("rogue_cert", to_pem(rogue_cert)),
                            ("rogue_key", to_key_pem(rogue_key))]:
            p = Path(tmp.name) / f"{name}.pem"
            p.write_bytes(value)
            paths[name] = str(p)

        server_ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        server_ctx.load_cert_chain(paths["server_cert"], paths["server_key"])
        server_ctx.verify_mode = ssl.CERT_REQUIRED
        server_ctx.load_verify_locations(cadata=to_pem(ca_cert).decode())

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        server.socket = server_ctx.wrap_socket(server.socket, server_side=True)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        port = server.server_address[1]

        client_ctx = ssl.create_default_context(cadata=to_pem(ca_cert).decode())
        client_ctx.load_cert_chain(paths["client_cert"], paths["client_key"])

        data = exchange(client_ctx, port, "elkeid.com", "demo-agent")
        assert data["registered"] == "demo-agent", data
        print("[1/3] 双向 TLS 握手成功，首包 agent_id 登记 OK")

        rogue_ctx = ssl.create_default_context(cadata=to_pem(ca_cert).decode())
        rogue_ctx.load_cert_chain(paths["rogue_cert"], paths["rogue_key"])
        try:
            exchange(rogue_ctx, port, "elkeid.com", "rogue-agent")
            raise AssertionError("未受信任客户端证书应被拒绝")
        except (ssl.SSLError, ConnectionResetError, OSError):
            print("[2/3] 未受信任客户端证书被握手拒绝 OK")

        try:
            exchange(client_ctx, port, "wrong.example", "demo-agent")
            raise AssertionError("非预期 DNS 名应被 Agent 侧拒绝")
        except ssl.SSLCertVerificationError:
            print("[3/3] 非预期 DNS 名被证书校验拒绝 OK")

        print("PASS: Elkeid 双向 TLS 认证过程 3/3 步通过")
    finally:
        server.shutdown()
        tmp.cleanup()


if __name__ == "__main__":
    main()
