package main

import (
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/brandonweeks/nanoca"
	nullauthorizer "github.com/brandonweeks/nanoca/authorizers/null"
	"github.com/brandonweeks/nanoca/issuers/inprocess"
	filesigner "github.com/brandonweeks/nanoca/signers/file"
	badgerstorage "github.com/brandonweeks/nanoca/storage/badger"
	appleverifier "github.com/brandonweeks/nanoca/verifiers/apple"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// Version is set at build time via -ldflags.
var Version = "dev"

func run() error {
	var (
		listen     = flag.String("listen", ":9003", "listen address")
		caCertF    = flag.String("ca-cert", "rootCA.crt", "CA certificate file")
		caKeyF     = flag.String("ca-key", "rootCA-pkcs8.key", "CA private key file (PKCS#8 PEM)")
		certF      = flag.String("cert", "", "TLS server certificate (leave empty to use CA cert)")
		keyF       = flag.String("key", "", "TLS server private key (leave empty to use CA key)")
		baseURL    = flag.String("base-url", "https://mdm.example.com", "external base URL")
		prefix     = flag.String("prefix", "/acme", "ACME URL prefix")
		storageDir = flag.String("storage", "", "BadgerDB storage directory (empty = in-memory)")
	)
	flag.Parse()

	logger := slog.New(nanoca.NewContextHandler(slog.Default().Handler()))

	caCert, err := loadCertificate(*caCertF)
	if err != nil {
		return fmt.Errorf("loading CA certificate: %w", err)
	}

	signer, err := filesigner.LoadSigner(*caKeyF)
	if err != nil {
		return fmt.Errorf("loading CA key: %w", err)
	}

	var storageOpts badgerstorage.Options
	if *storageDir == "" {
		storageOpts = badgerstorage.Options{InMemory: true}
		log.Println("Using in-memory storage (data lost on restart)")
	} else {
		storageOpts = badgerstorage.Options{Path: *storageDir}
		log.Printf("Using persistent storage: %s", *storageDir)
	}

	acmeStorage, err := badgerstorage.New(storageOpts)
	if err != nil {
		return fmt.Errorf("creating storage: %w", err)
	}

	ca, err := nanoca.New(
		logger,
		inprocess.New(caCert, signer),
		nullauthorizer.New(),
		acmeStorage,
		*baseURL,
		nanoca.WithPrefix(*prefix),
		nanoca.WithVerifier(appleverifier.New(logger)),
	)
	if err != nil {
		return fmt.Errorf("creating CA: %w", err)
	}
	defer ca.Close()

	mux := http.NewServeMux()
	mux.Handle("/", ca.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("NanoCA %s starting on %s", Version, *listen)
	log.Printf("ACME directory: %s%s/directory", *baseURL, *prefix)

	tlsCert := *certF
	tlsKey := *keyF
	if tlsCert == "" {
		tlsCert = *caCertF
	}
	if tlsKey == "" {
		tlsKey = *caKeyF
	}

	return http.ListenAndServeTLS(*listen, tlsCert, tlsKey, mux) //nolint:gosec
}

func loadCertificate(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", path)
	}
	return x509.ParseCertificate(block.Bytes)
}
