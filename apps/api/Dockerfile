FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /compass .
FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 compass
USER compass
COPY --from=build /compass /usr/local/bin/compass
EXPOSE 8080
ENTRYPOINT ["compass"]
