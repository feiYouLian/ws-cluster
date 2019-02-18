FROM golang:latest

WORKDIR $GOPATH/src/github.com/ws-cluster/
COPY . $GOPATH/src/github.com/ws-cluster/
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
RUN go get -u github.com/golang/dep/cmd/dep
RUN dep ensure
RUN go build -o ws-cluster .

EXPOSE 8180

ENTRYPOINT ["./ws-cluster"]
