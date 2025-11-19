#!/bin/bash
# Start OffGrid server with proxy settings

export HTTP_PROXY="http://127.0.0.1:11053"
export HTTPS_PROXY="http://127.0.0.1:11053"
export NO_PROXY="192.168.*,172.31.*,172.30.*,172.29.*,172.28.*,172.27.*,172.26.*,172.25.*,172.24.*,172.23.*,172.22.*,172.21.*,172.20.*,172.19.*,172.18.*,172.17.*,172.16.*,10.*,127.*,localhost"

echo "Starting OffGrid server with proxy support..."
cd /var/lib/offgrid
./offgrid serve
