# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
  go build -trimpath -ldflags="-s -w" -o /out/backup ./cmd/backup

FROM rclone/rclone:1.75.2

ARG USER_NAME="backuptool"
ARG USER_ID="1100"

ENV LOCALTIME_FILE="/tmp/localtime"

COPY --from=builder /out/backup /app/backup
COPY scripts/*.sh /app/

RUN chmod +x /app/*.sh /app/backup \
  && mkdir -m 777 /bitwarden \
  && apk add --no-cache 7zip bash curl mariadb-client postgresql18-client sqlite supercronic s-nail tzdata \
  && apk info --no-cache -Lq mariadb-client | grep -vE '/bin/mariadb$' | grep -vE '/bin/mariadb-dump$' | xargs -I {} rm -f "/{}" \
  && ln -sf "${LOCALTIME_FILE}" /etc/localtime \
  && addgroup -g "${USER_ID}" "${USER_NAME}" \
  && adduser -u "${USER_ID}" -Ds /bin/sh -G "${USER_NAME}" "${USER_NAME}"

ENTRYPOINT ["/app/entrypoint.sh"]
