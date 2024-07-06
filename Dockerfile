FROM gcr.io/distroless/base-nossl-debian12:nonroot

COPY size-it /

CMD ["/size-it"]
