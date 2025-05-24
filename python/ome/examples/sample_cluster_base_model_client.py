#!/usr/bin/env python3
"""
Sample client code for OME API - ClusterBaseModel operations
This example demonstrates how to use the OME client to create, get, update, and delete a ClusterBaseModel.
"""

import json

from kubernetes import client

from ome import (
    OMEClient,
    V1beta1BaseModelSpec,
    V1beta1ClusterBaseModel,
    V1beta1ModelFormat,
    V1beta1ModelFrameworkSpec,
    V1beta1StorageSpec,
    constants,
)
from ome.utils.logger import logger


def create_cluster_base_model(name):
    """
    Create a new cluster base model
    """
    ome_client = OMEClient()

    logger.info(f"Creating cluster base model {name}...")
    api_version = constants.OME_V1BETA1

    # Define node selector terms similar to DeepSeek-V3-0324.yaml
    node_selector_terms = [
        {
            "matchExpressions": [
                {
                    "key": "node.kubernetes.io/instance-type",
                    "operator": "In",
                    "values": ["BM.GPU.H100.8"],
                }
            ]
        }
    ]

    # Define model configuration based on DeepSeek-V3-0324.yaml
    model_config = {
        "architectures": ["DeepseekV3ForCausalLM"],
        "attention_bias": False,
        "attention_dropout": 0.0,
        "auto_map": {
            "AutoConfig": "configuration_deepseek.DeepseekV3Config",
            "AutoModel": "modeling_deepseek.DeepseekV3Model",
            "AutoModelForCausalLM": "modeling_deepseek.DeepseekV3ForCausalLM",
        },
        "hidden_size": 7168,
        "model_type": "deepseek_v3",
        "max_position_embeddings": 163840,
        "num_attention_heads": 128,
        "num_hidden_layers": 61,
    }

    # Create the cluster base model spec
    cluster_bm_spec = V1beta1ClusterBaseModel(
        api_version=api_version,
        kind=constants.OME_KIND_CLUSTERBASEMODEL,
        metadata=client.V1ObjectMeta(
            name=name,
        ),
        spec=V1beta1BaseModelSpec(
            vendor="deepseek-ai",
            disabled=False,
            version="1.0.0",
            compartment_id="ocid1.compartment.oc1..aaaaaaaathgntpo75bdehisnl6wkxfc4slkd6rpheafbt5a6ekm2ri4bmeva",
            model_format=V1beta1ModelFormat(name="safetensors", version="1"),
            model_framework=V1beta1ModelFrameworkSpec(
                name="transformers", version="4.33.1"
            ),
            model_architecture="DeepseekV3ForCausalLM",
            model_parameter_size="685B",
            max_tokens=163840,
            model_capabilities=["TEXT_GENERATION"],
            storage=V1beta1StorageSpec(
                storage_uri="oci://n/idqj093njucb/b/model-store/o/deepseek-ai/deepseek-v3",
                path=f"/raid/models/deepseek-ai/{name}",
                node_affinity=client.V1NodeAffinity(
                    required_during_scheduling_ignored_during_execution=client.V1NodeSelector(
                        node_selector_terms=node_selector_terms
                    )
                ),
            ),
            # model_configuration=json.dumps(model_config),
        ),
    )

    response = ome_client.cluster_base_model.create(
        clusterbasemodel=cluster_bm_spec,
    )

    # Pretty-print the JSON response
    pretty_response = json.dumps(response, indent=2)
    logger.info(f"Cluster base model details:\n{pretty_response}")
    return response


def get_cluster_base_model(name):
    """
    Get an existing cluster base model
    """
    ome_client = OMEClient()

    logger.info(f"Getting cluster base model {name}...")
    response = ome_client.cluster_base_model.get(
        name=name,
    )

    # Pretty-print the JSON response
    pretty_response = json.dumps(response, indent=2)
    logger.info(f"Cluster base model details:\n{pretty_response}")
    return response


def check_cluster_base_model_status(name):
    """
    Check if a cluster base model is ready
    """
    ome_client = OMEClient()

    logger.info(f"Checking if cluster base model {name} is ready...")
    is_ready = ome_client.cluster_base_model.is_ready(
        name=name,
    )

    logger.info(f"Cluster base model {name} ready status: {is_ready}")
    return is_ready


def wait_for_cluster_base_model_ready(name, timeout_seconds=600):
    """
    Wait for a cluster base model to be ready
    """
    ome_client = OMEClient()

    logger.info(f"Waiting for cluster base model {name} to be ready...")
    ome_client.cluster_base_model.wait_ready(
        name=name,
        timeout_seconds=timeout_seconds,
    )


def list_all_cluster_base_models():
    """
    List all cluster base models
    """
    ome_client = OMEClient()

    logger.info("Listing all cluster base models...")
    response = ome_client.cluster_base_model.get()

    # Get the list of cluster base models
    cluster_base_models = response.get("items", [])

    logger.info(f"Found {len(cluster_base_models)} cluster base models:")
    for model in cluster_base_models:
        name = model["metadata"]["name"]
        state = model.get("status", {}).get("state", "Unknown")
        logger.info(f"  - {name}: {state}")

    return response


def update_cluster_base_model(name, disabled=True):
    """
    Update an existing cluster base model to enable/disable it
    """
    ome_client = OMEClient()

    logger.info(f"Getting current cluster base model {name}...")
    current_model = ome_client.cluster_base_model.get(name=name)

    # Update the disabled field
    current_model["spec"]["disabled"] = disabled

    logger.info(f"Updating cluster base model {name} (disabled={disabled})...")
    response = ome_client.cluster_base_model.replace(
        name=name,
        clusterbasemodel=current_model,
    )

    # Pretty-print the JSON response
    pretty_response = json.dumps(response, indent=2)
    logger.info(f"Updated cluster base model details:\n{pretty_response}")
    return response


def delete_cluster_base_model(name):
    """
    Delete a cluster base model
    """
    ome_client = OMEClient()

    logger.info(f"Deleting cluster base model {name}...")
    response = ome_client.cluster_base_model.delete(
        name=name,
    )

    # Pretty-print the JSON response
    pretty_response = json.dumps(response, indent=2)
    logger.info(f"Delete response:\n{pretty_response}")
    return response


def main():
    """
    Main function to demonstrate the OME client usage for ClusterBaseModel operations
    """
    model_name = "sample-deepseek-model"

    try:
        delete_cluster_base_model(model_name)
        # Create a new cluster base model
        create_cluster_base_model(model_name)

        # Wait for it to be ready
        # wait_for_cluster_base_model_ready(model_name)

        # Get the cluster base model details
        get_cluster_base_model(model_name)

        # Check the status
        check_cluster_base_model_status(model_name)

        # List all cluster base models
        list_all_cluster_base_models()

        # Update the cluster base model to disable it
        update_cluster_base_model(model_name, disabled=True)

        # Get the updated cluster base model
        get_cluster_base_model(model_name)

        # Uncomment to delete the cluster base model
        delete_cluster_base_model(model_name)

    except Exception as e:
        logger.error(f"Error: {e}")


if __name__ == "__main__":
    main()
