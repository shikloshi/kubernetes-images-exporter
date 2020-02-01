FROM golang:1.13.7 AS builder
WORKDIR /root
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

FROM alpine:latest  
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /root/app .
CMD ["/root/app"]  
