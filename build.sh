GOOS=linux GOARCH=amd64 go build -ldflags="-X 'main.Version=$(cat Version)'"
