#!/usr/bin/env python3
"""
Sample client code for OME API
This example demonstrates how to use the OME client to create, get, update, and delete an inference service.
"""

import json
import time

from kubernetes import client

from ome import (
    OMEClient,
    V1beta1InferenceService,
    V1beta1InferenceServiceSpec,
    V1beta1ModelSpec,
    V1beta1PredictorSpec,
    constants,
)
from ome.utils.logger import logger


def create_inference_service(name, namespace="default"):
    """
    Create a new inference service
    """
    ome_client = OMEClient()

    logger.info(f"Creating inference service {name}...")
    api_version = constants.OME_V1BETA1
    isvc_spec = V1beta1InferenceService(
        api_version=api_version,
        kind=constants.OME_KIND_INFERENCESERVICE,
        metadata=client.V1ObjectMeta(
            name=name,
            namespace=namespace,
        ),
        spec=V1beta1InferenceServiceSpec(
            predictor=V1beta1PredictorSpec(
                min_replicas=1,
                max_replicas=1,
                model=V1beta1ModelSpec(
                    base_model="e5-mistral-7b-instruct",
                    protocol_version="openAI",
                ),
            )
        ),
    )
    response = ome_client.inference_service.create(
        inferenceservice=isvc_spec,
        namespace=namespace,
    )

    # Pretty-print the JSON response
    pretty_response = json.dumps(response, indent=2)
    logger.info(f"Inference service details:\n{pretty_response}")
    return response


def get_inference_service(name, namespace="default"):
    """
    Get an existing inference service
    """
    ome_client = OMEClient()

    logger.info(f"Getting inference service {name}...")
    response = ome_client.inference_service.get(
        name=name,
        namespace=namespace,
    )

    # Pretty-print the JSON response
    pretty_response = json.dumps(response, indent=2)
    logger.info(f"Inference service details:\n{pretty_response}")
    return response


def check_inference_service_status(name, namespace="default"):
    """
    Check if an inference service is ready
    """
    ome_client = OMEClient()

    logger.info(f"Checking if inference service {name} is ready...")
    is_ready = ome_client.inference_service.is_ready(
        name=name,
        namespace=namespace,
    )

    logger.info(f"Inference service {name} ready status: {is_ready}")
    return is_ready


def wait_for_inference_service_ready(name, namespace="default", timeout_seconds=300):
    """
    Wait for an inference service to be ready
    """
    ome_client = OMEClient()

    logger.info(f"Waiting for inference service {name} to be ready...")
    ome_client.inference_service.wait_ready(
        name=name,
        namespace=namespace,
        timeout_seconds=timeout_seconds,
        polling_interval=10,
    )

    logger.info(f"Inference service {name} is now ready")


def list_all_inference_services(namespace="default"):
    """
    List all inference services in a namespace
    """
    ome_client = OMEClient()

    logger.info(f"Listing all inference services in namespace {namespace}...")
    response = ome_client.inference_service.get(
        namespace=namespace,
    )

    services = response.get("items", [])
    logger.info(f"Found {len(services)} inference services")

    for svc in services:
        logger.info(f"  - {svc['metadata']['name']}")

    return response


def main():
    """
    Main function to demonstrate the OME client usage
    """
    try:
        # Wait a moment for the service to initialize
        time.sleep(5)

        isvc_name = "e5-mistral-7b-instruct"
        namespace = "default"

        create_inference_service(name=isvc_name, namespace=namespace)

        # Get the inference service
        get_inference_service(
            name=isvc_name,
            namespace=namespace,
        )

        # Check if the service is ready
        check_inference_service_status(
            name=isvc_name,
            namespace=namespace,
        )

        # Wait for the service to be ready
        wait_for_inference_service_ready(
            name=isvc_name,
            namespace=namespace,
        )

    except Exception as e:
        logger.error(f"Error: {e}")


if __name__ == "__main__":
    main()
