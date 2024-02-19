BUILD_DATE=$(date --iso-8601=seconds -u)

LDFLAGS="-X 'main.buildDate=${BUILD_DATE}' -s -w"

GOOS=windows GOARCH=amd64 go build -o build/cfddns-windows-amd64.exe -ldflags="${LDFLAGS}" ./cmd/cfddns
GOOS=linux GOARCH=amd64 go build -o build/cfddns-linux-amd64 -ldflags="${LDFLAGS}" ./cmd/cfddns
GOOS=linux GOARCH=arm64 go build -o build/cfddns-linux-arm64 -ldflags="${LDFLAGS}" ./cmd/cfddns
