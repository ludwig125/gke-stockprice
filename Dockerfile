FROM golang:1.13-alpine as builder

RUN mkdir /gke-stockprice
WORKDIR /gke-stockprice

# Install git + SSL ca certificates.
# Git is required for fetching the dependencies.
# Ca-certificates is required to call HTTPS endpoints.
RUN apk add --update --no-cache ca-certificates git tzdata && update-ca-certificates
#RUN apk add --update --no-cache ca-certificates

# COPY go.mod and go.sum files to the workspace
COPY go.mod .
COPY go.sum .

# Get dependancies - will also be cached if we won't change mod/sum
RUN go mod download
RUN go mod verify
# COPY the source code as the last step
COPY . .

# Build the binary
# race detector: https://golang.org/doc/articles/race_detector.html
RUN GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /go/bin/gke-stockprice

# Second step to build minimal image
#FROM scratch
FROM golang:1.13-alpine
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /go/bin/gke-stockprice /go/bin/gke-stockprice
# RUN apk add --update --no-cache tzdata && \
#     cp /usr/share/zoneinfo/Asia/Tokyo /etc/localtime && \
#     echo "Asia/Tokyo" > /etc/timezone && \
#     apk del tzdata
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
ENV TZ=Asia/Tokyo
ENTRYPOINT ["/go/bin/gke-stockprice"]
