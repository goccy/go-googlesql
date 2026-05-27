GOOGLESQL_WASM_REPO    ?= goccy/googlesql-wasm
GOOGLESQL_WASM_VERSION ?= v0.2.1

TARBALL    := googlesql_wasm2go.tar.gz
SHA256SUMS := googlesql_wasm2go.sha256
RELEASE_URL = https://github.com/$(GOOGLESQL_WASM_REPO)/releases/download/$(GOOGLESQL_WASM_VERSION)

.PHONY: googlesql download verify verify-release verify-attestation test

## googlesql: download release artifacts and verify their attestations.
googlesql: download verify

## download: fetch the wasm2go tarball + sha256 manifest and extract them
## into the repo root. The tarball is discarded once unpacked.
download:
	curl -fSL --proto '=https' --tlsv1.2 -o $(TARBALL)    $(RELEASE_URL)/$(TARBALL)
	curl -fSL --proto '=https' --tlsv1.2 -o $(SHA256SUMS) $(RELEASE_URL)/$(SHA256SUMS)
	tar xzf $(TARBALL)
	rm -f $(TARBALL)

## verify: ensure the in-tree files match the sha256 manifest AND that
## the manifest carries a valid GitHub release attestation. Either
## check failing aborts.
verify: verify-attestation verify-release

## verify-attestation: verify the GitHub release attestation for the
## sha256 manifest. The release attestation lists every published
## asset as a subject, so a valid signature over $(SHA256SUMS) is
## sufficient to trust its contents (no GH access token required).
verify-attestation:
	@echo "==> verifying GitHub release attestation for $(SHA256SUMS)"
	GH_TOKEN= GITHUB_TOKEN= gh release verify-asset $(GOOGLESQL_WASM_VERSION) \
	    -R $(GOOGLESQL_WASM_REPO) \
	    $(SHA256SUMS)

## verify-release: confirm every in-tree file matches the entries in
## $(SHA256SUMS). Run after verify-attestation, never before.
verify-release:
	@echo "==> verifying in-tree files against $(SHA256SUMS)"
	@shasum -a 256 -c $(SHA256SUMS)
	@echo "    OK in-tree files match $(GOOGLESQL_WASM_VERSION) release"

## test: run the Go test suite.
test:
	go test ./...
