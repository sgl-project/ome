#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Enable debug output
DEBUG=${DEBUG:-0}

# Debug function
debug() {
  if [ "$DEBUG" -eq 1 ]; then
    echo -e "${BLUE}[DEBUG] $1${NC}"
  fi
}

echo -e "${BLUE}Starting mock servers in the background...${NC}"
# Start mock servers in background and save PID
cargo run --example mock_servers > mock_servers.log 2>&1 &
MOCK_PID=$!

# Function to clean up resources on exit
cleanup() {
  echo -e "${YELLOW}Shutting down mock servers (PID: $MOCK_PID)...${NC}"
  kill $MOCK_PID 2>/dev/null || true
  
  if [ ! -z "$ROUTER_PID" ]; then
    echo -e "${YELLOW}Shutting down router (PID: $ROUTER_PID)...${NC}"
    kill $ROUTER_PID 2>/dev/null || true
  fi
  
  echo -e "${GREEN}Cleanup complete${NC}"
}

# Set up trap to ensure cleanup on script exit
trap cleanup EXIT

# Wait for mock servers to start
echo -e "${BLUE}Waiting for mock servers to initialize...${NC}"
MAX_WAIT=10
COUNT=0
while [ $COUNT -lt $MAX_WAIT ]; do
  if grep -q "Started mock server" mock_servers.log; then
    echo -e "${GREEN}Mock servers have started${NC}"
    break
  fi
  # Check for errors
  if grep -q "Error\|error\|Failed\|failed\|panic" mock_servers.log; then
    echo -e "${RED}Error detected while starting mock servers:${NC}"
    cat mock_servers.log
    exit 1
  fi
  echo -e "${YELLOW}Waiting for mock servers to start ($((COUNT+1))/$MAX_WAIT)...${NC}"
  sleep 1
  COUNT=$((COUNT+1))
done

if [ $COUNT -eq $MAX_WAIT ]; then
  echo -e "${RED}Mock servers did not start in time. Check log:${NC}"
  cat mock_servers.log
  exit 1
fi

# Test mock servers log
if [ "$DEBUG" -eq 1 ]; then
  echo -e "${BLUE}[DEBUG] Mock servers log:${NC}"
  cat mock_servers.log
fi

# Extract URLs from the log file
echo -e "${BLUE}Reading server URLs from log...${NC}"
CLASS_URL=$(grep "Classification server" mock_servers.log | awk '{print $NF}')
TEXT_URL=$(grep "Text processing server" mock_servers.log | awk '{print $NF}')
IMAGE_URL=$(grep "Image processing server" mock_servers.log | awk '{print $NF}')

if [ -z "$CLASS_URL" ] || [ -z "$TEXT_URL" ] || [ -z "$IMAGE_URL" ]; then
  echo -e "${RED}Failed to extract server URLs. Check mock_servers.log for details.${NC}"
  echo -e "${YELLOW}Log content:${NC}"
  cat mock_servers.log
  echo -e "${RED}Terminating due to missing server URLs${NC}"
  exit 1
fi

echo -e "${GREEN}Mock servers running at:${NC}"
echo -e "  Classification: ${BLUE}$CLASS_URL${NC}"
echo -e "  Text Processing: ${BLUE}$TEXT_URL${NC}"
echo -e "  Image Processing: ${BLUE}$IMAGE_URL${NC}"

# Test if mock servers are reachable
for url in "$CLASS_URL" "$TEXT_URL" "$IMAGE_URL"; do
  echo -e "${BLUE}Testing connectivity to $url...${NC}"
  if ! curl -s -f --max-time 5 -X POST -H "Content-Type: application/json" -d '{"test":true}' "$url" > /dev/null; then
    echo -e "${RED}Failed to connect to $url${NC}"
    echo -e "${YELLOW}This might cause the router to hang when processing requests${NC}"
    
    # Try to diagnose the issue
    echo -e "${BLUE}Running diagnostics...${NC}"
    echo -e "${YELLOW}Network interfaces:${NC}"
    ifconfig | grep inet
    
    echo -e "${YELLOW}Testing with localhost URLs instead...${NC}"
    url_port=$(echo "$url" | sed -E 's|http://[^:]+:([0-9]+).*|\1|')
    if [ ! -z "$url_port" ]; then
      localhost_url="http://localhost:$url_port"
      echo -e "${BLUE}Testing connectivity to $localhost_url...${NC}"
      if curl -s -f --max-time 5 -X POST -H "Content-Type: application/json" -d '{"test":true}' "$localhost_url" > /dev/null; then
        echo -e "${GREEN}Successfully connected to $localhost_url${NC}"
        # If localhost works, update the URL
        case "$url" in
          "$CLASS_URL") CLASS_URL="$localhost_url" ;;
          "$TEXT_URL") TEXT_URL="$localhost_url" ;;
          "$IMAGE_URL") IMAGE_URL="$localhost_url" ;;
        esac
      else
        echo -e "${RED}Failed to connect to $localhost_url as well${NC}"
      fi
    fi
  else
    echo -e "${GREEN}Successfully connected to $url${NC}"
  fi
done

# Create test graph configuration
GRAPH=$(cat <<EOF
{
  "nodes": {
    "root": {
      "routerType": "switch",
      "steps": [
        {
          "stepName": "process-text",
          "nodeName": "text-processing",
          "condition": "type.text"
        },
        {
          "stepName": "process-image",
          "nodeName": "image-processing",
          "condition": "type.image"
        }
      ]
    },
    "text-processing": {
      "routerType": "sequence",
      "steps": [
        {
          "stepName": "classify-content",
          "serviceUrl": "$CLASS_URL",
          "dependency": "hard"
        },
        {
          "stepName": "process-text",
          "serviceUrl": "$TEXT_URL",
          "data": "\$response",
          "dependency": "soft"
        }
      ]
    },
    "image-processing": {
      "routerType": "sequence",
      "steps": [
        {
          "stepName": "classify-content",
          "serviceUrl": "$CLASS_URL",
          "dependency": "hard"
        },
        {
          "stepName": "process-image",
          "serviceUrl": "$IMAGE_URL",
          "data": "\$response",
          "dependency": "soft"
        }
      ]
    }
  }
}
EOF
)

# Save the graph to a file for reference
echo "$GRAPH" > test_graph.json
echo -e "${GREEN}Test graph configuration:${NC}"
echo "$GRAPH" | jq '.'

# Start the router with the test graph in the background
echo -e "${BLUE}Starting inference router with test graph in background...${NC}"
# Set a short timeout for the router to prevent hanging
export RUST_BACKTRACE=1
cargo run -- --graph-string "$GRAPH" --port 8080 --host 0.0.0.0 > router.log 2>&1 &
ROUTER_PID=$!

# Wait for the router to start
echo -e "${BLUE}Waiting for router to start...${NC}"
MAX_RETRIES=30
RETRY_COUNT=0
ROUTER_URL="http://localhost:8080"

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
  if curl -s -f "$ROUTER_URL/health" > /dev/null 2>&1; then
    echo -e "${GREEN}Router is running at $ROUTER_URL${NC}"
    break
  fi
  echo -e "${YELLOW}Waiting for router to start (attempt $((RETRY_COUNT+1))/$MAX_RETRIES)...${NC}"
  sleep 1
  RETRY_COUNT=$((RETRY_COUNT+1))
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
  echo -e "${RED}Router did not start after $MAX_RETRIES attempts. Check router.log for details.${NC}"
  cat router.log
  exit 1
fi

# Function to send requests to the router and display results with timeout
test_request() {
  local name=$1
  local payload=$2
  local expected=$3
  local timeout_seconds=5

  echo -e "\n${BLUE}Test: $name${NC}"
  echo -e "${YELLOW}Sending request:${NC}"
  echo "$payload" | jq '.' || echo "$payload"
  
  echo -e "${YELLOW}Response (with ${timeout_seconds}s timeout):${NC}"
  response=$(timeout ${timeout_seconds}s curl -s -X POST \
    -H "Content-Type: application/json" \
    -H "X-Request-ID: test-$RANDOM" \
    -d "$payload" \
    $ROUTER_URL || echo '{"error":"Request timed out after '${timeout_seconds}' seconds"}')
  
  # Check if the response is empty or contains "Request timed out"
  if [ -z "$response" ] || [[ "$response" == *"Request timed out"* ]]; then
    echo -e "${RED}Request timed out after ${timeout_seconds} seconds${NC}"
    echo -e "${YELLOW}Checking router logs for errors...${NC}"
    tail -n 20 router.log
    return 1
  fi
  
  echo "$response" | jq '.' || echo "$response"
  
  if [[ "$response" == *"$expected"* ]]; then
    echo -e "${GREEN}Test passed!${NC}"
    return 0
  else
    echo -e "${RED}Test failed - Expected to find '$expected' in response${NC}"
    return 1
  fi
}

# Track overall test status
TEST_STATUS=0

# Test 1: Text processing
echo -e "\n${BLUE}==============================================${NC}"
echo -e "${BLUE}Testing text processing route${NC}"
echo -e "${BLUE}==============================================${NC}"
if ! test_request "Text Processing" '{"type":{"text":true},"content":"Hello world"}' "processed"; then
  TEST_STATUS=1
fi

# Test 2: Image processing
echo -e "\n${BLUE}==============================================${NC}"
echo -e "${BLUE}Testing image processing route${NC}"
echo -e "${BLUE}==============================================${NC}"
if ! test_request "Image Processing" '{"type":{"image":true},"width":100,"height":100}' "dimensions"; then
  TEST_STATUS=1
fi

# Test 3: Invalid route (no matching condition)
echo -e "\n${BLUE}==============================================${NC}"
echo -e "${BLUE}Testing invalid route (no matching condition)${NC}"
echo -e "${BLUE}==============================================${NC}"
if ! test_request "Invalid Route" '{"type":{"video":true}}' "error"; then
  TEST_STATUS=1
fi

# Test 4: Empty payload
echo -e "\n${BLUE}==============================================${NC}"
echo -e "${BLUE}Testing empty payload${NC}"
echo -e "${BLUE}==============================================${NC}"
if ! test_request "Empty Payload" '{}' "error"; then
  TEST_STATUS=1
fi

if [ $TEST_STATUS -eq 0 ]; then
  echo -e "\n${GREEN}All tests completed successfully!${NC}"
else
  echo -e "\n${RED}Some tests failed!${NC}"
  echo -e "${YELLOW}Router log:${NC}"
  cat router.log
fi

# Show logs if requested
if [ "$1" == "--show-logs" ]; then
  echo -e "\n${BLUE}Router logs:${NC}"
  cat router.log
fi

# Keep servers running if requested
if [ "$1" == "--keep-running" ]; then
  echo -e "\n${YELLOW}Servers are still running. Press Enter to stop them...${NC}"
  read
else
  echo -e "\n${BLUE}Shutting down servers...${NC}"
fi

exit $TEST_STATUS 