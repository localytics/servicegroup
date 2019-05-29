FROM golang:1.12.5-stretch AS base
# Alpine (musl-based) cannot run race detector currently: https://github.com/golang/go/issues/14481
RUN apt-get update && apt-get -y install rsync

ENV CGO_ENABLED=0
ENV GO111MODULE=on

# use docker-compose to mount local source over /code
WORKDIR /code

# this entrypoint/cmd are only hit from compose
ENTRYPOINT ["go"]
CMD ["test", "-v", "./..."]

# keep going to actually build a binary into an image
FROM base AS build

# Front-load module downloads so Docker can cache these layers even if source code changes.
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . /code
RUN go build -a -o servicegroup-example-server cmd/servicegroup-example-server/main.go
# Copy necessary source files to /code.

# Final runtime container: copy just our binary into a clean Alpine image.
FROM alpine:3.9 as runtime
COPY --from=build /code/servicegroup-example-server ./

# Expose debug/pprof HTTP server on 6060
EXPOSE 6060/tcp
# Expose application HTTP server on 8080
EXPOSE 8080/tcp
ENTRYPOINT ["./servicegroup-example-server"]