FROM golang:1.13-buster as builder

# setup the working directory
WORKDIR /go/src/github.com/yasoobh/loadtest

# add files required for go mod download
ADD ./go.sum ./go.sum
ADD ./go.mod ./go.mod

# install dependencies
RUN go mod download

# add source code
ADD . .

# why is this needed?
# create system dependency files
# RUN echo "hosts: files dns" > /etc/nsswitch.conf

# build the source
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o loadtest

########################################
## Production Stage
########################################
FROM ubuntu:20.04

# set working directory
WORKDIR /root

# copy required files from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/nsswitch.conf /etc/nsswitch.conf
COPY --from=builder /go/src/github.com/yasoobh/loadtest/loadtest ./loadtest


ENTRYPOINT ["./loadtest"]
CMD ["-tf", "data/target.txt"]
