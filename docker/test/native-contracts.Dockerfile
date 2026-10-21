FROM ubuntu:24.04
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    build-essential pkg-config ca-certificates git \
    libatspi2.0-dev libgtk-3-dev libjson-glib-dev \
    at-spi2-core dbus-x11 xvfb xauth openbox \
    && rm -rf /var/lib/apt/lists/*
RUN useradd --create-home --uid 1100 native-test
USER native-test
ENV PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
ENV GOCACHE=/tmp/offgrid-go-cache GOMODCACHE=/tmp/offgrid-go-modules
ENV GOMAXPROCS=4 GOFLAGS=-p=4
WORKDIR /work
