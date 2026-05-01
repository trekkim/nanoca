# nanoca

A lightweight enterprise [ACME](https://datatracker.ietf.org/doc/html/rfc8555) Certificate Authority service with [device attestation](https://datatracker.ietf.org/doc/draft-ietf-acme-device-attest/) support. It provides just the HTTP handlers needed to implement ACME, it is intended to be integrated into [nanomdm](https://github.com/micromdm/nanomdm) or another service of your choosing. Storage, signing, authorization, and logging are implemented as pluggable interfaces to integrate into a wide variety of environments.

Upstream: [github.com/brandonweeks/nanoca](https://github.com/brandonweeks/nanoca)

---

## Trend Micro Fork Changes

This fork adds a standalone `cmd/nanoca` binary and Go 1.25 compatibility fixes so NanoCA can run as an independent ACME CA server alongside MicroMDM.

### Changes

| File | Change |
|------|--------|
| `cmd/nanoca/main.go` | **NEW** — standalone HTTP server binary. Flags: `-ca-cert`, `-ca-key`, `-base-url`, `-prefix`, `-listen`, `-storage`. TLS served directly using the CA cert/key. |
| `errorscompat.go` | **NEW** — Go 1.25 compatibility shim: local `asType[T]` generic replacing `errors.AsType[T]` (not available until Go 1.26). |
| `handlers.go` | Replaced `errors.AsType[T]` calls with local `asType[T]`. |
| `jose.go` | Replaced `errors.AsType[T]` calls with local `asType[T]`. |
| `nanoca.service` | **NEW** — systemd unit file for running NanoCA as a service. |
| `nginx-acme.conf` | **NEW** — reference nginx location block for proxying `/acme/` to NanoCA. |

### Why

MicroMDM fork (`micromdm-main`) proxies `/acme/*` requests to NanoCA via `-acme-backend` flag. NanoCA implements the `device-attest-01` ACME challenge with Apple's Enterprise Attestation Root CA embedded — it verifies Apple hardware attestations and issues device identity certificates with serial number and UDID in SAN extensions.

---

## Library Usage

```go
import (
	"github.com/brandonweeks/nanoca"
	"github.com/brandonweeks/nanoca/authorizers/null"
	"github.com/brandonweeks/nanoca/issuers/inprocess"
	"github.com/brandonweeks/nanoca/signers/file"
	"github.com/brandonweeks/nanoca/storage/badger"
)

signer, _ := file.LoadSigner("rootCA.key")
storage, _ := badger.New(badger.Options{InMemory: true})

ca, _ := nanoca.New(
	inprocess.New(signer),
	null.New(),
	storage,
	"https://localhost:8443",
	nanoca.WithPrefix("/acme"),
)
defer ca.Close()

mux := http.NewServeMux()
mux.Handle("/", ca.Handler())
```

---

## Standalone Server

### Build

Requires Go 1.25+.

```bash
# Build for current platform
go build -ldflags "-X main.Version=v1.0.0" -o nanoca ./cmd/nanoca

# Build for Linux (amd64)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
  -ldflags "-X main.Version=v1.0.0" \
  -o build/nanoca ./cmd/nanoca
```

### PKI Setup

```bash
# Generate Root CA (EC P-256, 10 years)
openssl ecparam -name prime256v1 -genkey -noout -out pki/rootCA.key
openssl pkcs8 -topk8 -nocrypt -in pki/rootCA.key -out pki/rootCA-pkcs8.key
openssl req -new -x509 -days 3650 \
  -key pki/rootCA-pkcs8.key \
  -out pki/rootCA.crt \
  -subj "/O=Your Org/CN=MDM Device CA" \
  -addext "basicConstraints=critical,CA:TRUE"
```

`pki/rootCA.key` and `pki/rootCA-pkcs8.key` are excluded from git via `.gitignore`. Only `pki/rootCA.crt` (public cert) may be committed.

### Run

```bash
./nanoca \
  -ca-cert pki/rootCA.crt \
  -ca-key pki/rootCA-pkcs8.key \
  -base-url https://mdm.example.com \
  -prefix /acme \
  -listen :9003 \
  -storage /var/lib/nanoca/data
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `-ca-cert` | `rootCA.crt` | CA certificate (PEM) |
| `-ca-key` | `rootCA-pkcs8.key` | CA private key (PKCS#8 PEM) |
| `-base-url` | `https://mdm.example.com` | External base URL for ACME directory |
| `-prefix` | `/acme` | ACME URL path prefix |
| `-listen` | `:9003` | Listen address |
| `-storage` | (empty) | BadgerDB directory; empty = in-memory |
| `-cert` | (empty) | TLS server cert; defaults to `-ca-cert` |
| `-key` | (empty) | TLS server key; defaults to `-ca-key` |

### Systemd

See [nanoca.service](nanoca.service). Copy to `/etc/systemd/system/nanoca.service`, adjust paths, then:

```bash
systemctl daemon-reload
systemctl enable --now nanoca
```

### Nginx Proxy (optional)

If running behind nginx instead of exposing NanoCA directly, see [nginx-acme.conf](nginx-acme.conf) for the required `location /acme/` proxy block.

When using MicroMDM with `-acme-backend=https://localhost:9003`, nginx is not required — MicroMDM proxies ACME requests directly.

---

## Known Limitations

### akd attestation map

NanoCA validates Apple device attestation via `device-attest-01` challenge but issues certificates signed by **your custom Root CA**. macOS `akd` requires the MDM identity certificate to be signed by **Apple's Enterprise Attestation Sub CA** for the attestation map to be populated.

Without the attestation map entry, `akd` skips the `GetToken` MDM flow and falls back to browser-based Managed Apple ID sign-in (PSSO/Entra ID web flow).

Apple's own ACME endpoint (`deviceenrollment.apple.com/acme/enrollment`) is not accessible for third-party MDM servers at this time.
