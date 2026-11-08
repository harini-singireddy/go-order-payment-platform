FROM golang:1.27-alpine AS builder
WORKDIR /src

COPY go.work go.work.sum ./
COPY pkg/ pkg/
COPY proto/ proto/
COPY services/gateway/ services/gateway/
COPY services/order/ services/order/
COPY services/inventory/ services/inventory/
COPY services/payment/ services/payment/
COPY services/notification/ services/notification/
COPY proto/ proto/
COPY services/payment/ services/payment/

RUN cd services/payment && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/payment ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -g '' appuser
COPY --from=builder /out/payment /payment
USER appuser
EXPOSE 8082
ENTRYPOINT ["/payment"]
