FROM golang:1.27.3 AS build

WORKDIR /go/src/app

COPY . .
RUN CGO_ENABLED=0 GO111MODULE=on go install .
RUN mkdir /sessions

FROM ghcr.io/greboid/dockerbase/nonroot:1.20261020.1
COPY --from=build /go/bin/contact-form /contact-form
COPY --from=build /go/src/app/templates /templates
COPY --from=build /go/src/app/static /static
COPY --from=build --chown=65532:65532 /sessions /sessions

WORKDIR /
VOLUME /sessions
ENTRYPOINT ["/contact-form", "--session-path", "/sessions/sessions.db"]
EXPOSE 8080
