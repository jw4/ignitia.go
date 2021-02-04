#
# Builder
#

FROM    golang:1.15 AS builder

RUN     apt-get update && apt-get -uy upgrade
RUN     apt-get -y install ca-certificates && update-ca-certificates

WORKDIR /src
COPY    . . 

ARG     GOPROXY
ENV     CGO_ENABLED=0 \
        GOPROXY=${GOPROXY}

RUN     go build \
           -tags=netgo \
           -ldflags '-s -w -extldflags "-static"' \
           -o /ignitia ./cmd/ignitia

#
# Image
#

FROM    scratch

LABEL   maintainer="John Weldon <john@tempusbreve.com>" \
        company="Tempus Breve Software" \
        description="Ignitia Report Site"

COPY    --from=builder /etc/ssl/certs /etc/ssl/certs
COPY    --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY    --from=builder /ignitia /ignitia

COPY    public /var/html/

ENV     IGNITIA_BASE_URL="https://ignitiumwa.ignitiaschools.com" \
        IGNITIA_USERNAME="" \
        IGNITIA_PASSWORD="" \
        BIND=":80" \
        PUBLIC_ASSETS="/var/html/" \
        TZ="America/Phoenix"

EXPOSE  80/tcp

ENTRYPOINT ["/ignitia", "serve"]

