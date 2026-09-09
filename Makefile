.PHONY: run test fmt vet check paper

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
