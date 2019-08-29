FROM golang:1.12

WORKDIR /app
COPY . $GOPATH/src/github.com/ws-cluster/
COPY ./conf.ini /app/conf.ini
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
RUN export GO111MODULE=on
RUN export GOPROXY=https://goproxy.io
RUN go build -o /app/ws-cluster $GOPATH/src/github.com/ws-cluster/
EXPOSE 8380

ENTRYPOINT ["./ws-cluster"]
