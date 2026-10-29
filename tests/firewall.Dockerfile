ARG BASE=ubuntu:24.04
FROM ${BASE}
ARG MANAGER=ufw
RUN if [ "$MANAGER" = ufw ]; then \
      apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ufw nftables iproute2 iputils-ping wireguard-tools curl python3 procps ca-certificates; \
    else \
      dnf install -y firewalld nftables iproute iputils wireguard-tools curl-minimal python3 procps-ng dbus-daemon; \
    fi
