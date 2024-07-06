FROM gcr.io/distroless/static-debian12:nonroot

COPY size-it /

CMD ["/size-it"]
