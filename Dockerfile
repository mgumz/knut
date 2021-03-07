##
## -- build environment
##

FROM    golang:1.16.0-alpine3.13 AS build-env

ADD     . /src/github.com/mgumz/knut
RUN     apk add -U --no-cache make git
RUN     make -C /src/github.com/mgumz/knut simple

##
## -- runtime environment
##

FROM    alpine:3.13 AS rt-env

COPY    --from=build-env /src/github.com/mgumz/knut/bin/knut /knut

EXPOSE  8080
ENTRYPOINT ["/knut"]
