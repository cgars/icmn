.PHONY: run test fmt vet check paper diagrams

run:
	go run ./cmd/icmn

test:
	go test ./...

fmt:
	gofmt -w $$(find . -name '*.go' -type f)

vet:
	go vet ./...

check:
	test -z "$$(gofmt -l $$(find . -name '*.go' -type f))"
	go vet ./...
	go test -race ./...

paper:
	cd paper && pdflatex -interaction=nonstopmode -halt-on-error icmn.tex

diagrams:
	command -v drawio >/dev/null
	drawio --export --format svg --output docs/architecture/icmn-architecture.svg docs/architecture/icmn-architecture.drawio
	drawio --export --format png --scale 2 --output docs/architecture/icmn-architecture.png docs/architecture/icmn-architecture.drawio
