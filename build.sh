#!/bin/bash
export GOPATH=$PWD
env GOOS=linux GOARCH=386 go build -o bin/te-ips-linux te-ips
#env GOOS=linux GOARCH=amd64 go build -o bin/te-ips-linux-amd64 te-ips
#env GOOS=linux GOARCH=arm go build -o bin/te-ips-linux-arm te-ips
env GOOS=darwin GOARCH=amd64 go build -o bin/te-ips-macos te-ips
env GOOS=windows GOARCH=386 go build -o bin/te-ips.exe te-ips
