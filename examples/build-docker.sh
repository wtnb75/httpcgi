#! /bin/sh

prefix=tmp/

docker build -t ${prefix}hello -f ./Dockerfile.hello .
docker build -t ${prefix}listfiles -f ./Dockerfile.listfiles .
