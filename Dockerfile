FROM golang:1.25.2 AS build

WORKDIR /go/src/app

COPY . .
RUN CGO_ENABLED=0 GO111MODULE=on go install .
RUN mkdir /sessions

FROM gcr.io/distroless/base:nonroot
COPY --from=build /go/bin/contact-form /contact-form
COPY --from=build /go/src/app/templates /templates
COPY --from=build /go/src/app/static /static
COPY --from=build --chown=nonroot /sessions /sessions

WORKDIR /
VOLUME /sessions
ENTRYPOINT ["/contact-form", "--session-path", "/sessions/sessions.db"]
EXPOSE 8080
