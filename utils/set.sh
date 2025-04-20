#!/bin/bash

# this script allows consult and set the breakers parameters

# flags -e is the environment, it can be local, ci, uat or production (default is local)

# The parameter are (all are optional). If the parameter is not provided the script will get every parameter value
# -m memory: the memory threshold in %
# -l latency: the latency threshold in ms
# -s size: the latency window size
# -p percentile: the latency percentile in %
# -t wait: the wait time in seconds
# -a action: can be 'get' or 'set'
# -v value: the value to set (used with -a set)

usage() {
    echo "Usage: $0 [-e <env>] [-a <action>] [-m [<value>]] [-l [<value>]] [-s [<value>]] [-p [<value>]] [-t [<value>]] [-v <value>]" 1>&2
    echo "  -e <env>       : Environment (local, ci, uat, production). Default: local" 1>&2
    echo "  -a <action>    : Action to perform (get, set). Default: get" 1>&2
    echo "  -m [<value>]   : Memory threshold parameter with optional value" 1>&2
    echo "  -l [<value>]   : Latency threshold parameter with optional value" 1>&2
    echo "  -s [<value>]   : Latency window size parameter with optional value" 1>&2
    echo "  -p [<value>]   : Percentile parameter with optional value" 1>&2
    echo "  -t [<value>]   : Wait time parameter with optional value" 1>&2
    echo "  -v <value>     : Value to set (alternative way)" 1>&2
    echo "" 1>&2
    echo "Examples:" 1>&2
    echo "  $0 -a get -m                        # Get memory threshold" 1>&2
    echo "  $0 -a set -m 80                     # Set memory threshold to 80%" 1>&2
    echo "  $0 -a set -m -v 80                  # Same as above" 1>&2
    echo "  $0 -a set -l 200                    # Set latency threshold to 200ms" 1>&2
    exit 1
}

# Default values
env="local"
action="get"
value=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        -e)
            env="$2"
            shift 2
            ;;
        -a)
            action="$2"
            shift 2
            ;;
        -m)
            param="memory"
            # Check if the next argument is a value or another flag
            if [[ -n "$2" && "$2" != -* ]]; then
                value="$2"
                shift 2
            else
                shift
            fi
            ;;
        -l)
            param="latency"
            # Check if the next argument is a value or another flag
            if [[ -n "$2" && "$2" != -* ]]; then
                value="$2"
                shift 2
            else
                shift
            fi
            ;;
        -s)
            param="size"
            # Check if the next argument is a value or another flag
            if [[ -n "$2" && "$2" != -* ]]; then
                value="$2"
                shift 2
            else
                shift
            fi
            ;;
        -p)
            param="percentile"
            # Check if the next argument is a value or another flag
            if [[ -n "$2" && "$2" != -* ]]; then
                value="$2"
                shift 2
            else
                shift
            fi
            ;;
        -t)
            param="wait"
            # Check if the next argument is a value or another flag
            if [[ -n "$2" && "$2" != -* ]]; then
                value="$2"
                shift 2
            else
                shift
            fi
            ;;
        -v)
            value="$2"
            shift 2
            ;;
        -h)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

# Validate action
if [ "$action" != "get" ] && [ "$action" != "set" ]; then
    echo "Error: Action must be 'get' or 'set'"
    usage
fi

# Validate environment
if [ "$env" != "local" ] && [ "$env" != "ci" ] && [ "$env" != "uat" ] && [ "$env" != "production" ]; then
    echo "Error: Invalid environment"
    usage
fi

# Set base URL
if [ "$env" == "local" ]; then
    base_url="http://localhost:8080"
else
    # read the environment variable SPORT and LEAGUE
    sport=$SPORT
    league=$LEAGUE
    # build the URL
    base_url="http://${sport}-mc-${league}.sports-models-${env}.i.geniussports.com"
fi

# Add breaker path to URL
url="${base_url}/breaker"

# Function to perform GET request
perform_get() {
    local endpoint=$1
    echo "Getting $endpoint"
    curl -X GET -H "Content-Type: application/json" "$url/$endpoint"
    echo
}

# Function to perform POST request with JSON payload
perform_post() {
    local endpoint=$1
    local json_data=$2
    echo "Setting $endpoint with value: $json_data"
    curl -X POST -H "Content-Type: application/json" -d "$json_data" "$url/$endpoint"
    echo
}

# Handle the action
if [ "$action" == "get" ]; then
    # GET actions
    case "$param" in
        "memory")
            perform_get "memory"
            ;;
        "latency")
            perform_get "latency"
            ;;
        "size")
            perform_get "latency_window_size"
            ;;
        "percentile")
            perform_get "percentile"
            ;;
        "wait")
            perform_get "wait"
            ;;
        *)
            # If no specific parameter is provided, get all
            echo "Getting all parameters"
            perform_get "memory"
            perform_get "latency"
            perform_get "latency_window_size"
            perform_get "percentile"
            perform_get "wait"
            ;;
    esac
elif [ "$action" == "set" ]; then
    # SET actions - require value
    if [ -z "$value" ]; then
        echo "Error: Value is required for set action"
        usage
    fi

    case "$param" in
        "memory")
            perform_post "set_memory" "{\"threshold\": $value}"
            ;;
        "latency")
            perform_post "set_latency" "{\"threshold\": $value}"
            ;;
        "size")
            perform_post "set_latency_window_size" "{\"size\": $value}"
            ;;
        "percentile")
            perform_post "set_percentile" "{\"percentile\": $value}"
            ;;
        "wait")
            perform_post "set_wait" "{\"wait_time\": $value}"
            ;;
        *)
            echo "Error: You must specify a parameter to set"
            usage
            ;;
    esac
fi
