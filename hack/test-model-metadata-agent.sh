#!/bin/bash
# Example of how the BaseModel controller would invoke the model-metadata agent

# This would be run as a Kubernetes Job with the model PVC mounted at /model
ome-agent model-metadata \
  --config /etc/ome-agent/model-metadata.yaml \
  --model-path /model \
  --basemodel-name llama-7b \
  --basemodel-namespace model-serving \
  --cluster-scoped false