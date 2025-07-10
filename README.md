# DuckDuckGo Search API

A lightweight, containerized REST API service that provides search functionality by proxying requests to DuckDuckGo.

## Features

- Simple REST API for searching DuckDuckGo
- Basic authentication for API security
- Configurable result limits
- Health check endpoint
- Docker and Kubernetes ready
- Local development mode

## Configuration

The service can be configured using environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Port the server listens on | `8080` |
| `AUTH_USERNAME` | Username for Basic Auth | Required |
| `AUTH_PASSWORD` | Password for Basic Auth | Required |
| `DEBUG` | Enable debug logging | `false` |
| `LOCAL_MODE` | Bypass authentication for local testing | `false` |

## API Endpoints

### Search

```
GET /search?q=your+search+query&limit=10
```

Parameters:

- `q`: Search query (required)
- `limit`: Maximum number of results to return (optional)

Authentication:

- Basic Authentication required (unless in LOCAL_MODE)

Response:

```json
[
  {
    "title": "Result title",
    "url": "https://example.com",
    "snippet": "Result description..."
  }
]
```

### Health Check

```
GET /health
```

Response: `OK` with status code 200

## Running Locally

```bash
# Set required environment variables
export AUTH_USERNAME=user
export AUTH_PASSWORD=pass
export LOCAL_MODE=true

# Run the service
go run main.go
```

## Docker

Build and run with Docker:

```bash
# Build the image
docker build -t home-go-api-template .

# Run the container
docker run -p 8080:8080 \
  -e AUTH_USERNAME=user \
  -e AUTH_PASSWORD=pass \
  home-go-api-template
```

## Kubernetes Deployment

Kubernetes manifests are provided in the `k8s` directory:

```bash
# Apply the Kubernetes resources
kubectl apply -k k8s/
```

## Development

### Prerequisites

- Go 1.24

### Testing

```bash
go test ./...
```

## License

[WTFPL - Do What The Fuck You Want To Public License](http://www.wtfpl.net/)
