FROM golang:1.22.2 as build

WORKDIR /go/src/updater
COPY . .

RUN go mod download
RUN go vet -v
RUN go test -v

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /go/bin/updater

FROM golang:1.22.2
COPY --from=build /go/bin/updater /

ENV GIN_MODE=release
CMD ["/updater"]