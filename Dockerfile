FROM node:22-alpine AS web
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS server
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY --from=web /web/dist ./webdist
RUN CGO_ENABLED=0 go build -o /nodemon-server .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 nodemon && mkdir /data && chown nodemon /data
USER nodemon
ENV DB_PATH=/data/nodemon.db ADDR=:8080
COPY --from=server /nodemon-server /usr/local/bin/nodemon-server
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["nodemon-server"]
