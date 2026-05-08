GOOGLESQL_WASM_REPO     ?= goccy/googlesql-wasm
GOOGLESQL_WASM_VERSION  ?= v0.1.5
GOOGLESQL_WASM_WORKFLOW ?= goccy/googlesql-wasm/.github/workflows/build.yml

ARTIFACTS       ?= googlesql.go googlesql.wasm
RELEASE_URL      = https://github.com/$(GOOGLESQL_WASM_REPO)/releases/download/$(GOOGLESQL_WASM_VERSION)
ATTESTATION_API  = https://api.github.com/repos/$(GOOGLESQL_WASM_REPO)/attestations

.PHONY: googlesql download verify verify-release verify-attestation test clean-bundles

## googlesql: download release artifacts and verify their attestations.
googlesql: download verify

## download: fetch googlesql.{go,wasm} from the goccy/googlesql-wasm release.
download:
	curl -fSL --proto '=https' --tlsv1.2 -o googlesql.go   $(RELEASE_URL)/googlesql.go
	curl -fSL --proto '=https' --tlsv1.2 -o googlesql.wasm $(RELEASE_URL)/googlesql.wasm

## verify: ensure the in-tree googlesql.{go,wasm} match the upstream release
## bytewise AND carry valid GitHub attestations. Either check failing aborts.
verify: verify-release verify-attestation

## verify-release: byte-for-byte compare the in-tree files against the release.
verify-release:
	@set -eu; \
	tmp=$$(mktemp -d); \
	trap "rm -rf $$tmp" EXIT; \
	for f in $(ARTIFACTS); do \
	  curl -fsSL --proto '=https' --tlsv1.2 -o $$tmp/$$f $(RELEASE_URL)/$$f; \
	  if ! cmp -s $$f $$tmp/$$f; then \
	    echo "ERROR: in-tree $$f differs from $(GOOGLESQL_WASM_VERSION) release"; \
	    echo "  in-tree: $$(shasum -a 256 $$f | awk '{print $$1}')"; \
	    echo "  release: $$(shasum -a 256 $$tmp/$$f | awk '{print $$1}')"; \
	    exit 1; \
	  fi; \
	  echo "    OK $$f matches $(GOOGLESQL_WASM_VERSION) release"; \
	done

## verify-attestation: verify GitHub artifact attestations (no GH access token required).
verify-attestation:
	@set -eu; for f in $(ARTIFACTS); do \
	  digest=$$(shasum -a 256 $$f | awk '{print $$1}'); \
	  echo "==> attesting $$f (sha256:$$digest)"; \
	  curl -fsSL --proto '=https' --tlsv1.2 \
	    "$(ATTESTATION_API)/sha256:$$digest" \
	    | jq '.attestations[].bundle' > $$f.bundle.jsonl; \
	  GH_TOKEN= GITHUB_TOKEN= gh attestation verify $$f \
	    -R $(GOOGLESQL_WASM_REPO) \
	    --bundle $$f.bundle.jsonl \
	    --signer-workflow $(GOOGLESQL_WASM_WORKFLOW); \
	  echo "    OK $$f"; \
	  rm -f $$f.bundle.jsonl; \
	done

## test: run the Go test suite.
test:
	go test ./...

clean-bundles:
	rm -f googlesql.go.bundle.jsonl googlesql.wasm.bundle.jsonl
