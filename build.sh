#!/bin/bash
export GOPATH=$PWD
env GOOS=linux GOARCH=386 go build -o bin/linux-32/te-ips te-ips
env GOOS=linux GOARCH=amd64 go build -o bin/linux-64/te-ips te-ips
env GOOS=linux GOARCH=arm go build -o bin/linux-arm/te-ips te-ips
env GOOS=darwin GOARCH=amd64 go build -o bin/macos/te-ips te-ips
env GOOS=windows GOARCH=386 go build -o bin/win/te-ips.exe te-ips
