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

# git + cgit power the git:// and cgit:// handlers (git http-backend / cgit
# are invoked as CGI subprocesses).
RUN     apk upgrade --no-cache && \
        apk add --no-cache git cgit tini-static

COPY    bin/knut-${VERSION}.linux.${TARGETARCH} /knut

EXPOSE  8080
ENTRYPOINT ["/sbin/tini", "--", "/knut"]
