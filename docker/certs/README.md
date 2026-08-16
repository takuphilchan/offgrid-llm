# TLS certificates

Production nginx expects `cert.pem` and `key.pem` in this directory. Do not
commit a private key. Install certificates issued for your hostname before
starting `docker-compose.prod.yml`.
