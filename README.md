# Stream Puller API

A lightweight, containerized REST API service for downloading video and audio streams using `yt-dlp` and `ffmpeg`.

## Features

- Download video streams from various sources.
- Download audio streams from various sources.
- Configurable output formats, resolutions, and codecs.
- Basic authentication for API security.
- Health check endpoint.
- Docker and Kubernetes ready.
- Local development mode.

## Configuration

The service can be configured using environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Port the server listens on | `8080` |
| `AUTH_USERNAME` | Username for Basic Auth | Required |
| `AUTH_PASSWORD` | Password for Basic Auth | Required |
| `DEBUG` | Enable debug logging | `false` |
| `LOCAL_MODE` | Bypass authentication for local testing | `false` |
| `YTDLP_PATH` | Path to the `yt-dlp` executable | `yt-dlp` |
| `FFMPEG_PATH` | Path to the `ffmpeg` executable | `ffmpeg` |

## API Endpoints

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
# Optional: Set custom paths if yt-dlp/ffmpeg are not in your PATH
# export YTDLP_PATH=/usr/local/bin/yt-dlp
# export FFMPEG_PATH=/usr/local/bin/ffmpeg

# Run the service
go run main.go
```

## Docker

Build and run with Docker:

```bash
# Build the image
docker build -t gostreampuller .

# Run the container
docker run -p 8080:8080 \
  -e AUTH_USERNAME=user \
  -e AUTH_PASSWORD=pass \
  gostreampuller
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
- `yt-dlp` and `ffmpeg` installed and accessible in your system's PATH or specified via environment variables.

### Testing

```bash
go test ./...
```

## License

[WTFPL - Do What The Fuck You Want To Public License](http://www.wtfpl.net/)
