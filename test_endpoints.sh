#!/bin/bash

echo "Testing Claude Proxy Endpoint Management APIs"
echo "=============================================="

BASE_URL="http://127.0.0.1:8081/admin/api"

echo
echo "1. Testing GET /admin/api/endpoints"
curl -s "$BASE_URL/endpoints" | head -3

echo
echo
echo "2. Testing POST /admin/api/endpoints (Create new endpoint)"
curl -s -X POST "$BASE_URL/endpoints" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-endpoint",
    "url": "https://test.example.com",
    "path_prefix": "/v1",
    "auth_type": "api_key",
    "auth_value": "test-key-123",
    "enabled": true
  }' | head -3

echo
echo
echo "3. Testing GET /admin/api/endpoints (after create)"
curl -s "$BASE_URL/endpoints" | head -3

echo
echo
echo "4. Testing PUT /admin/api/endpoints/test-endpoint (Update endpoint)"
curl -s -X PUT "$BASE_URL/endpoints/test-endpoint" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://updated.example.com",
    "enabled": false
  }' | head -3

echo
echo
echo "5. Testing POST /admin/api/endpoints/reorder (Reorder endpoints)"
curl -s -X POST "$BASE_URL/endpoints/reorder" \
  -H "Content-Type: application/json" \
  -d '{
    "ordered_names": ["test-endpoint", "mirrorcode", "gac"]
  }' | head -3

echo
echo
echo "6. Testing DELETE /admin/api/endpoints/test-endpoint (Delete endpoint)"
curl -s -X DELETE "$BASE_URL/endpoints/test-endpoint" | head -3

echo
echo
echo "API Testing completed!"