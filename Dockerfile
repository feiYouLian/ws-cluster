FROM golang:latest

WORKDIR /app
COPY . $GOPATH/src/github.com/ws-cluster/
COPY ./conf.ini /app/conf.ini
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
# RUN go get -u github.com/golang/dep/cmd/dep
# RUN dep ensure
RUN go build -o /app/ws-cluster $GOPATH/src/github.com/ws-cluster/
EXPOSE 8380

ENTRYPOINT ["./ws-cluster"]
