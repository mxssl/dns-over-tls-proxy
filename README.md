# WIP

# dns-over-tls-proxy

DNS to DNS-over-TLS proxy. This proxy accepts plain DNS queries (UDP and TCP) on one side and translates it to DNS-over-TLS queries on the other side.

## Features

- UDP and TCP support
- In-memory cache
- Structured logs (JSON)

## How to run this app

1. Build a container:

```bash
docker-compose build
```

2. Start a container

```bash
docker-compose up -d
```

## How to check this app

- TCP resolver:

```bash
dig @localhost -p 5053 +tcp google.com
```

- UDP resolver:

```bash
dig @localhost -p 5053 google.com
```
