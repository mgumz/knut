##
## the binary is built outside of the image - by "make container-image"
## locally, by the workflow in .github/workflows with the go which mise
## tracks in .mise/config.toml - and copied in as it is:
##
##   bin/knut-$VERSION.linux.$TARGETARCH
##
## TARGETARCH is set by buildx / podman from --platform.
##

FROM    alpine:3.24

ARG     VERSION=dev
ARG     TARGETARCH

# git powers the git:// handler (git http-backend runs as a CGI
# subprocess).
RUN     apk upgrade --no-cache && \
        apk add --no-cache git tini-static

COPY    bin/knut-${VERSION}.linux.${TARGETARCH} /knut

EXPOSE  8080
ENTRYPOINT ["/sbin/tini", "--", "/knut"]
