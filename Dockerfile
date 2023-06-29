FROM golang:alpine AS builder

WORKDIR /build

ADD go.mod .
ADD go.sum .
COPY . .

RUN go build -ldflags="-s -w" -o main main.go

FROM alpine
RUN apk update --no-cache && apk add --no-cache ca-certificates tzdata
ENV TZ Asia/Shanghai

WORKDIR /app

COPY --from=builder /build ./
# COPY --from=builder /build/etc ./etc

CMD ["./main"]