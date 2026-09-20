##
## -- build environment
##

FROM    golang:1.27-alpine3.24 AS build-env

ARG     VERSION=dev

WORKDIR /src
RUN     apk upgrade --no-cache && \
        apk add -U --no-cache git

# cache deps first
COPY    go.mod go.sum ./
COPY    vendor ./vendor
RUN     go mod verify

COPY    . .
RUN     CGO_ENABLED=0 go build -trimpath \
            -ldflags "-X github.com/mgumz/knut/internal/pkg/knut.Version=$VERSION \
                      -X github.com/mgumz/knut/internal/pkg/knut.GitHash=$VERSION" \
            -o bin/knut ./cmd/knut

##
## -- runtime environment
##

FROM    alpine:3.24 AS rt-env

# git + cgit power the git:// and cgit:// handlers (git http-backend / cgit
# are invoked as CGI subprocesses).
RUN     apk upgrade --no-cache && \
        apk add --no-cache git cgit tini-static

COPY    --from=build-env /src/bin/knut /knut

EXPOSE  8080
ENTRYPOINT ["/sbin/tini", "--", "/knut"]
