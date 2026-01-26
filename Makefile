.PHONY: run

run:
	go run ./cmd/open-next-router --config ./open-next-router.yaml

.PHONY: encrypt

encrypt:
	go run ./cmd/onr-crypt --text "$(TEXT)"
