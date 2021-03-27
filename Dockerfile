FROM golang:1.16 AS builder

WORKDIR /go/src/github.com/jamiely/sample

COPY *.go .
COPY go.* .

RUN go get ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o benchmark .
RUN chmod +x benchmark

FROM scratch

COPY --chown=1000:1000 --from=builder /go/src/github.com/jamiely/sample/benchmark /benchmark

ENTRYPOINT ["/benchmark"]

CMD ["-h"]