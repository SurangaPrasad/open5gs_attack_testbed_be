# Kubernetes Status API

A REST API built with Gin to query the status of specific Kubernetes pods in your Open5GS network. This API provides endpoints to monitor core network, access network, and monitoring pods.

## Project Structure

```
.
├── main.go              # Main application entry point
├── handlers/            # HTTP handlers
│   └── pods.go         # Pod-related HTTP handlers
└── k8s/                # Kubernetes-related code
    ├── client.go       # Kubernetes client initialization
    └── pods.go         # Pod-related functions
```

## Prerequisites

- Go 1.21 or later
- Kubernetes cluster access (either running locally or remote)
- `kubectl` configured with access to your cluster

## Setup

1. Clone this repository
2. Install dependencies:
   ```bash
   go mod tidy
   ```

## Running the Application

### Local Development

Make sure you have your `kubectl` configured with the correct context. Then run:

```bash
go run main.go
```

The server will automatically find an available port (trying 8080-8083) and start on it.

### Running in Kubernetes

1. Build the container:
   ```bash
   docker build -t k8s-status-api .
   ```

2. Deploy to your Kubernetes cluster:
   ```bash
   kubectl apply -f k8s/
   ```

## API Endpoints

### GET /core-network
Returns a list of core network pods (pods with names starting with "open5gs").

Example response:
```json
{
  "pods": [
    {
      "name": "open5gs-amf",
      "namespace": "default",
      "containers": ["amf"],
      "status": "Running"
    }
  ]
}
```

### GET /access-network
Returns a list of access network pods (pods with names starting with "ueransim").

Example response:
```json
{
  "pods": [
    {
      "name": "ueransim-gnb",
      "namespace": "default",
      "containers": ["gnb"],
      "status": "Running"
    }
  ]
}
```

### GET /monitoring
Returns a list of monitoring pods (pods with names starting with "prometheus").

Example response:
```json
{
  "pods": [
    {
      "name": "prometheus-server",
      "namespace": "monitoring",
      "containers": ["prometheus"],
      "status": "Running"
    }
  ]
}
```

## Features

- Automatic port selection (8080-8083)
- CORS support for cross-origin requests
- Filtered pod listing by name prefix
- Detailed pod information including:
  - Pod name
  - Namespace
  - Container names
  - Pod status

## Security Note

This API provides read-only access to pod information. Make sure to:
1. Configure appropriate RBAC rules when deploying to Kubernetes
2. Use HTTPS in production
3. Implement authentication if needed

## Error Handling

The API includes proper error handling for:
- Kubernetes client initialization failures
- Pod listing failures
- Port binding conflicts

## Development

To modify the code:
1. The main application logic is in `main.go`
2. Kubernetes-related functions are in the `k8s` package
3. HTTP handlers are in the `handlers` package

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a new Pull Request 