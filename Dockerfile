FROM golang:1.20.5-alpine3.18 as build
WORKDIR /build
COPY go.* ./
RUN go mod download
COPY . .
RUN apk add --no-cache git
RUN go build -v -o app .

FROM alpine
WORKDIR /service
COPY --from=build /build/app .
COPY --from=build /build/assets ./assets
RUN apk add --no-cache tzdata ffmpeg
ENTRYPOINT ["./app"]
