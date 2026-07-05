FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN go test ./...
RUN go build -o /out/snippetsync ./cmd/server

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/snippetsync /app/snippetsync
EXPOSE 8080
CMD ["/app/snippetsync"]
