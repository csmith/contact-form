FROM golang:1.15 AS build

WORKDIR /go/src/app

COPY . .
RUN CGO_ENABLED=0 GO111MODULE=on go install .

FROM scratch
COPY --from=build /go/bin/contact-form /contact-form
COPY --from=build /go/src/app/*.html /templates/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

WORKDIR /templates
ENTRYPOINT ["/contact-form"]
EXPOSE 8080
