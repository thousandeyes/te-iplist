#!/bin/bash
export GOPATH=$PWD
env GOOS=linux GOARCH=386 go build -o bin/linux-32/te-iplist src/te-iplist/te-iplist.go
env GOOS=linux GOARCH=amd64 go build -o bin/linux-64/te-iplist src/te-iplist/te-iplist.go
env GOOS=linux GOARCH=arm go build -o bin/linux-arm/te-iplist src/te-iplist/te-iplist.go
env GOOS=darwin GOARCH=amd64 go build -o bin/macos/te-iplist src/te-iplist/te-iplist.go
env GOOS=darwin GOARCH=arm64 go build -o bin/macos-arm64/te-iplist src/te-iplist/te-iplist.go
env GOOS=windows GOARCH=386 go build -o bin/win/te-iplist.exe src/te-iplist/te-iplist.go
