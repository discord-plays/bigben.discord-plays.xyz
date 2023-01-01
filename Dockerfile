FROM golang:1.19

WORKDIR /go/src/app
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

CMD ["bigben.mrmelon54.com"]
