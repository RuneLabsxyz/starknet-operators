FROM alpine:latest
# Required tools
RUN apk add --no-cache rclone zstd coreutils curl jq
WORKDIR /app
COPY restore.sh restore.sh
RUN chmod +x restore.sh
ENTRYPOINT [ "/app/restore.sh" ]
