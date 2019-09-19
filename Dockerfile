FROM golang:1.12

WORKDIR /app
COPY . /app/
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
RUN export GO111MODULE=on
RUN go build -o wscluster
EXPOSE 8380

ENV PATH /app:$PATH
