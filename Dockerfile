##
## -- build environment
##

FROM    golang:1.16.0-alpine3.13 AS build-env

ARG     KNUT_BIN=bin/knut

ADD     . /src/knut
RUN     apk add -U --no-cache make git
RUN     make -C /src/knut $KNUT_BIN

##
## -- runtime environment
##

FROM    alpine:3.13 AS rt-env

ARG     KNUT_BIN=bin/knut
COPY    --from=build-env /src/github.com/mgumz/knut/knut /knut

EXPOSE  8080
ENTRYPOINT ["/knut"]
