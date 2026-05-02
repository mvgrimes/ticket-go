APP      := "tk"
VER_FILE  := "./cmd/tk/main.go"
MAIN_FILE := "./cmd/tk/main.go"
VERSION   := shell('perl -nE "m{version\\s*=\\s*\"(\\d+\\.\\d+\\.\\d+)\"}i && print \$1" ' + VER_FILE)

build:
  echo "Building verions {{VERSION}} of {{APP}}"
  go build -o {{APP}} {{MAIN_FILE}}
  # codesign --force --sign - {{APP}}

lint:
  go vet ./... || true
  golangci-lint run ./... || true
  govulncheck ./...

fmt:
  go fmt ./...

test:
  go test ./...

release:
  go mod tidy
  just fmt
  just build
  git diff --exit-code
  git tag --points-at HEAD | grep -qx {{VERSION}} || git tag {{VERSION}}
  git push
  git release
  git push --tags
  goreleaser release --clean
