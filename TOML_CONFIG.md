# TOML Configuration Guide for Gateway Multicaster

This document describes in detail the structure and configuration options for Gateway Multicaster using TOML files.

## What is TOML?

TOML (Tom's Obvious, Minimal Language) is an easy-to-read configuration file format designed to be a minimal configuration language that's semantically unambiguous. It is similar to INI but with clearer data types and structure.

## General Structure

The TOML file for Gateway Multicaster is organized into several sections:

1. `[service]` - General service configuration
2. `[kubernetes]` - Kubernetes-related configuration
3. `[response_aggregation]` - Response aggregation configuration
4. `[[endpoints]]` - Endpoint definitions (there can be multiple)
5. `[opsgenie]` - OpsGenie integration configuration

## `[service]` Section

Configures general parameters of the multicaster service.

```toml
[service]
name = "Gateway Multicaster"      # Service name
version = "1.0.0"                 # Service version
log_level = "info"                # Log level (debug, info, warn, error)
timeout_ms = 5000                 # Request timeout (in milliseconds)
retry_count = 3                   # Number of retries for failed requests
```

## `[kubernetes]` Section

Defines how to interact with the Kubernetes cluster and gateway pods.

```toml
[kubernetes]
gateway_selector = "my-gateway"   # Selector to find gateway pods (by name or label)
namespace = "default"             # Namespace where the pods are located
service_port = 8080               # Service port on the pods
```

## `[response_aggregation]` Section

Configures how responses from multiple gateways are aggregated.

```toml
[response_aggregation]
parallel_calls = true             # true for parallel calls, false for sequential
```

## `[[endpoints]]` Section

Defines the endpoints that will be available in the multicaster. You can define multiple endpoints with different configurations.

```toml
[[endpoints]]
name = "Get Status"               # Descriptive name of the endpoint
path = "/api/status"              # Endpoint path
method = "GET"                    # HTTP method (GET, POST, PUT, DELETE, PATCH)
description = "Get the gateway status" # Description for documentation

# Path parameters (optional)
[endpoints.path_params]
id = "string"                     # Format: parameter_name = "type"

# Query parameters (optional)
[endpoints.query_params]
filter = "string"                 # Parameter name and type
limit = 10                        # Integer values are inferred automatically
enabled = true                    # Boolean values are inferred automatically

# Body configuration (optional, for POST, PUT, PATCH)
[endpoints.body]
content_type = "application/json" # Content type
schema = '''
{
  "name": "string",
  "age": "number",
  "active": "boolean"
}
'''                               # JSON schema as a string
```

## `[opsgenie]` Section

Configures integration with OpsGenie for alerting when circuit breaker events occur.

```toml
[opsgenie]
enabled = true                           # Enable OpsGenie integration
region = "us"                            # OpsGenie region ("us" or "eu")
api_url = ""                             # Custom API URL (optional)
source = "go-breaker"                    # Alert source name
tags = ["production", "circuit-breaker"] # Tags to apply to alerts
team = "platform-team"                   # Team to assign alerts to
trigger_on_open = true                   # Send alerts when breaker opens
trigger_on_reset = true                  # Send alerts when breaker resets
trigger_on_memory = true                 # Send alerts on memory threshold breach
trigger_on_latency = true                # Send alerts on latency threshold breach
include_latency_metrics = true           # Include latency metrics in alerts
include_memory_metrics = true            # Include memory metrics in alerts
include_system_info = true               # Include system info in alerts
latency_threshold = 1500                 # Latency threshold for alerts (ms)
alert_cooldown_seconds = 300             # Minimum time between similar alerts
priority = "P2"                          # Default alert priority

# API Information - used to identify and describe the protected service
api_name = "Payment Service"             # Name of the API being protected
api_version = "v1.2.3"                   # Version of the API
api_namespace = "payment"                # Namespace/category of the API
api_description = "Handles payment processing"  # Description of the API
api_owner = "Payments Team"              # Owner/team responsible for the API
api_priority = "critical"                # Business priority of the API
api_dependencies = [                     # List of dependent services
  "user-service",
  "billing-service"
]
api_endpoints = [                        # List of important endpoints
  "/api/v1/payments",
  "/api/v1/refunds"
]

# Custom API attributes - any key-value pairs for additional context
[opsgenie.api_custom_attributes]
environment = "production"
datacenter = "us-east-1"
tier = "core-service"
```

## Complete Example

Here's a complete example of a TOML configuration file for Gateway Multicaster with OpsGenie integration:

```toml
[service]
name = "Gateway Multicaster"
version = "1.0.0"
log_level = "info"
timeout_ms = 5000
retry_count = 3

[kubernetes]
gateway_selector = "my-gateway"
namespace = "default"
service_port = 8080

[response_aggregation]
parallel_calls = true

[opsgenie]
enabled = true
region = "us"
source = "go-breaker"
tags = ["production", "payment-api"]
team = "platform-team"
trigger_on_open = true
trigger_on_reset = true
trigger_on_memory = true
trigger_on_latency = true
include_latency_metrics = true
include_memory_metrics = true
include_system_info = true
latency_threshold = 1500
alert_cooldown_seconds = 300
priority = "P2"

# API Information
api_name = "Payment API"
api_version = "v1.2.3"
api_namespace = "payment"
api_description = "Handles all payment processing"
api_owner = "Payments Team"
api_priority = "critical"
api_dependencies = [
  "user-service",
  "billing-service",
  "fraud-detection"
]
api_endpoints = [
  "/api/v1/payments",
  "/api/v1/refunds",
  "/api/v1/subscriptions"
]

[opsgenie.api_custom_attributes]
environment = "production"
datacenter = "us-east-1"
tier = "core-service"
pci_compliant = "true"

# Simple GET endpoint
[[endpoints]]
name = "Get Status"
path = "/api/status"
method = "GET"
description = "Get the gateway status"

# Endpoint with path parameters
[[endpoints]]
name = "Get User"
path = "/api/users/:id"
method = "GET"
description = "Get user information by ID"

[endpoints.path_params]
id = "string"

# Endpoint with query parameters
[[endpoints]]
name = "List Users"
path = "/api/users"
method = "GET"
description = "List users with optional filters"

[endpoints.query_params]
filter = "string"
limit = 10
active = true

# POST endpoint with body
[[endpoints]]
name = "Create User"
path = "/api/users"
method = "POST"
description = "Create a new user"

[endpoints.body]
content_type = "application/json"
schema = '''
{
  "name": "string",
  "email": "string",
  "role": "string",
  "age": "number",
  "active": "boolean"
}
'''

# PUT endpoint with path parameters and body
[[endpoints]]
name = "Update User"
path = "/api/users/:id"
method = "PUT"
description = "Update an existing user's information"

[endpoints.path_params]
id = "string"

[endpoints.body]
content_type = "application/json"
schema = '''
{
  "name": "string",
  "email": "string",
  "role": "string",
  "age": "number",
  "active": "boolean"
}
'''
```

## Configuration Validation

Gateway Multicaster automatically validates the configuration at startup. The following fields are mandatory:

- `service.name`
- `kubernetes.gateway_selector`
- `kubernetes.namespace`
- `kubernetes.service_port` (must be positive)
- At least one endpoint must be defined
- For each endpoint:
    - `name`
    - `path`
    - `method`
    - If body is defined, `content_type` is required

## Configuration Reload

Currently, Gateway Multicaster does not support hot reloading of configuration. To apply changes to the configuration, it is necessary to restart the service.

## Tips and Best Practices

1. **Descriptive names**: Use descriptive names for endpoints that clearly indicate their purpose
2. **Consistent paths**: Maintain a consistent style in paths (with or without leading slash)
3. **Clear documentation**: Provide clear descriptions for each endpoint
4. **Appropriate timeout**: Configure a reasonable timeout based on your gateways' characteristics
5. **Parallelism**: Enable `parallel_calls` for better performance with multiple gateways, unless you need to guarantee execution order