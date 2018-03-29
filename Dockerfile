# First build the application
FROM alpine
RUN apk --no-cache add go git musl-dev
RUN go get -u github.com/skx/dns-api-go

# Now put it in a container without all the build tools
FROM alpine
WORKDIR /root/
COPY --from=0 /root/go/bin/dns-api-go .
EXPOSE 9999
ENTRYPOINT ["/root/dns-api-go", "-host", "0.0.0.0", "-port", "9999" ]
