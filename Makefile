GOOGLESQL_WASM_REPO     ?= goccy/googlesql-wasm
GOOGLESQL_WASM_VERSION  ?= v0.2.1
GOOGLESQL_WASM_WORKFLOW ?= goccy/googlesql-wasm/.github/workflows/build.yml

TARBALL         := googlesql_wasm2go.tar.gz
SHA256SUMS      := googlesql_wasm2go.sha256
RELEASE_URL      = https://github.com/$(GOOGLESQL_WASM_REPO)/releases/download/$(GOOGLESQL_WASM_VERSION)
ATTESTATION_API  = https://api.github.com/repos/$(GOOGLESQL_WASM_REPO)/attestations

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

## verify: byte-check each in-tree file against the sha256 manifest AND
## confirm each carries a valid GitHub artifact attestation signed by
## the upstream build.yml workflow. Either check failing aborts.
verify: verify-release verify-attestation

## verify-release: confirm every in-tree file matches the entries in
## $(SHA256SUMS). Fast sanity check; not a trust anchor on its own.
verify-release:
	@echo "==> verifying in-tree files against $(SHA256SUMS)"
	@shasum -a 256 -c $(SHA256SUMS)

## verify-attestation: confirm every in-tree artifact is a signed
## subject of the upstream SLSA build attestation. The build emits one
## attestation whose subject list covers every file in the tarball, so
## we fetch the bundle once anonymously from the public attestation
## API and then offline-verify each file via `gh attestation verify
## --bundle`. No GH access token is required.
verify-attestation:
	@set -eu; \
	tmpdir=$$(mktemp -d); \
	bundle=$$tmpdir/bundle.jsonl; \
	trap 'rm -rf $$tmpdir' EXIT; \
	digest=$$(shasum -a 256 googlesql.go | awk '{print $$1}'); \
	echo "==> fetching attestation bundle for googlesql.go (sha256:$$digest)"; \
	curl -fsSL --proto '=https' --tlsv1.2 \
	  "$(ATTESTATION_API)/sha256:$$digest" \
	  | jq -c '.attestations[].bundle' > $$bundle; \
	files=$$(awk '{print $$2}' $(SHA256SUMS) | sed 's|^\./||'); \
	for f in $$files; do \
	  echo "==> verifying $$f"; \
	  GH_TOKEN= GITHUB_TOKEN= gh attestation verify "$$f" \
	    -R $(GOOGLESQL_WASM_REPO) \
	    --bundle $$bundle \
	    --signer-workflow $(GOOGLESQL_WASM_WORKFLOW); \
	done

## test: run the Go test suite.
test:
	go test ./...
