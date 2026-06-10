GOOGLESQL_WASM_REPO     ?= goccy/googlesql-wasm
GOOGLESQL_WASM_VERSION  ?= v0.3.1
GOOGLESQL_WASM_WORKFLOW ?= goccy/googlesql-wasm/.github/workflows/build.yml

BRIDGE_ASSET     := googlesql_wasm2go.go
BRIDGE_FILE      := googlesql.go
RELEASE_URL       = https://github.com/$(GOOGLESQL_WASM_REPO)/releases/download/$(GOOGLESQL_WASM_VERSION)
ATTESTATION_API   = https://api.github.com/repos/$(GOOGLESQL_WASM_REPO)/attestations

.PHONY: googlesql download verify test

## googlesql: refresh the wasm2go bridge from the upstream release
## and verify its GitHub artifact attestation. Default to running
## this whenever GOOGLESQL_WASM_VERSION bumps.
googlesql: download verify

## download: fetch the wasm2go-runtime bridge from the upstream
## release and drop it in place at $(BRIDGE_FILE). The release
## publishes it as $(BRIDGE_ASSET); the rename is purely cosmetic
## (gh attestation verify matches by content digest).
download:
	curl -fSL --proto '=https' --tlsv1.2 -o $(BRIDGE_FILE) $(RELEASE_URL)/$(BRIDGE_ASSET)

## verify: confirm $(BRIDGE_FILE) carries a valid GitHub artifact
## attestation signed by the upstream build.yml workflow. The
## attestation bundle is fetched anonymously from the public
## attestation API and verified offline via
## `gh attestation verify --bundle`. No GH access token is required.
verify:
	@set -eu; \
	tmpdir=$$(mktemp -d); \
	bundle=$$tmpdir/bundle.jsonl; \
	trap 'rm -rf $$tmpdir' EXIT; \
	digest=$$(shasum -a 256 $(BRIDGE_FILE) | awk '{print $$1}'); \
	echo "==> fetching attestation bundle for $(BRIDGE_FILE) (sha256:$$digest)"; \
	curl -fsSL --proto '=https' --tlsv1.2 \
	  "$(ATTESTATION_API)/sha256:$$digest" \
	  | jq -c '.attestations[].bundle' > $$bundle; \
	echo "==> verifying $(BRIDGE_FILE)"; \
	GH_TOKEN= GITHUB_TOKEN= gh attestation verify "$(BRIDGE_FILE)" \
	  -R $(GOOGLESQL_WASM_REPO) \
	  --bundle $$bundle \
	  --signer-workflow $(GOOGLESQL_WASM_WORKFLOW)

## test: run the Go test suite.
test:
	go test ./...
