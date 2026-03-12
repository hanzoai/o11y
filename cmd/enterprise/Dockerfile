FROM alpine:3.20.3
LABEL maintainer="hanzoai"
WORKDIR /root

ARG OS="linux"
ARG TARGETARCH

RUN apk update && \
	apk add ca-certificates && \
	rm -rf /var/cache/apk/*


COPY ./target/${OS}-${TARGETARCH}/signoz /root/o11y
COPY ./templates/email /root/templates
COPY frontend/build/ /etc/o11y/web/

RUN chmod 755 /root /root/o11y

ENTRYPOINT ["./o11y", "server"]
