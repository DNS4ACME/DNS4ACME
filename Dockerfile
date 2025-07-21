FROM golang AS builder
COPY . /work
WORKDIR /work
ENV CGO_ENABLED=0
RUN go build -tags kubernetes -o /work/dns4acme github.com/dns4acme/dns4acme/cmd/dns4acme

FROM scratch
COPY --from=builder /work/dns4acme /dns4acme
COPY LICENSE.md /
EXPOSE 5353/udp
EXPOSE 5353/tcp
ENTRYPOINT ["/dns4acme"]
