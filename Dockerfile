FROM golang:latest

WORKDIR /data
COPY . $GOPATH/src/github.com/ws-cluster/
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
# RUN go get -u github.com/golang/dep/cmd/dep
# RUN dep ensure
RUN go build -o /data/ws-cluster $GOPATH/src/github.com/ws-cluster/

EXPOSE 8380

ENTRYPOINT ["./ws-cluster"]
