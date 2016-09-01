#!/bin/bash
export GOPATH=$PWD
env GOOS=linux GOARCH=386 go build -o bin/linux-32/te-iplist te-iplist
env GOOS=linux GOARCH=amd64 go build -o bin/linux-64/te-iplist te-iplist
env GOOS=linux GOARCH=arm go build -o bin/linux-arm/te-iplist te-iplist
env GOOS=darwin GOARCH=amd64 go build -o bin/macos/te-iplist te-iplist
env GOOS=windows GOARCH=386 go build -o bin/win/te-iplist.exe te-iplist
