FROM alpine:latest

RUN apk add --update-cache tzdata
COPY be-position /be-position

ENTRYPOINT ["/be-position"]


