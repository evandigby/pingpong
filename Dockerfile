FROM golang:latest as builder
ARG appdir 
LABEL maintainer="Evan Digby <evdigby@microsoft.com>"

RUN mkdir -p /app/$appdir
WORKDIR /app

COPY go.mod .
COPY go.sum .
COPY $appdir/*.go ./$appdir/

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app ./$appdir

FROM alpine:latest  
RUN apk --no-cache add ca-certificates
WORKDIR /
COPY --from=builder ./app .
CMD ["/app"]