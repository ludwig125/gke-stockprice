FROM golang:1.13-alpine as builder

RUN mkdir /gke-stockprice
WORKDIR /gke-stockprice

# Install git + SSL ca certificates.
# Git is required for fetching the dependencies.
# Ca-certificates is required to call HTTPS endpoints.
#RUN apk add --update --no-cache ca-certificates git && update-ca-certificates
RUN apk add --update --no-cache ca-certificates

# COPY go.mod and go.sum files to the workspace
COPY go.mod .
COPY go.sum .

# Get dependancies - will also be cached if we won't change mod/sum
RUN go mod download
# COPY the source code as the last step
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/gke-stockprice

# Second step to build minimal image
#FROM scratch
FROM golang:1.13-alpine
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /go/bin/gke-stockprice /go/bin/gke-stockprice
RUN apk --no-cache add tzdata && \
    cp /usr/share/zoneinfo/Asia/Tokyo /etc/localtime && \
    apk del tzdata
ENTRYPOINT ["/go/bin/gke-stockprice"]
