# URL Shortener

A lightweight URL shortener built with Go and SQLite.

## Features

- Shorten URLs with auto-generated or custom short codes
- 7-day link expiration
- Click tracking & statistics
- Rate limiting (100 req/min per IP)
- Simple web interface

## Run

```bash
go run main.go
```

Open [http://localhost:8080](http://localhost:8080).

## API Endpoints

**Shorten a URL**

```
POST /shorten
{"url": "https://example.com", "custom_code": "my-link"}
```

**Redirect**

```
GET /{code}
```

**Get stats**

```
GET /stats/{code}
```
