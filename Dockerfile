# docker build --tag loadtest .
# docker run -v $(pwd)/target.txt:/root/target.txt loadtest

FROM golang:1.13-buster as builder

# add a label to clean up later
LABEL stage=intermediate

# setup the working directory
WORKDIR /go/src/github.com/Zomato/loadtest

# # add netrc file to allow access to private github repo
# ADD .netrc /root/.netrc

# # set up env for go mod
# ENV GO111MODULE=on

# add files required for go mod download
ADD ./go.sum ./go.sum
ADD ./go.mod ./go.mod

# install dependencies
RUN go mod download

# add source code
ADD . .

# ADD ./configs /root/configs

# why is this needed?
# create system dependency files
RUN echo "hosts: files dns" > /etc/nsswitch.conf

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
COPY --from=builder /go/src/github.com/Zomato/loadtest/loadtest ./loadtest
# COPY --from=builder /go/src/github.com/Zomato/search-service/configs ./configs


ENTRYPOINT ["./loadtest", "-targets", "target.txt"]
