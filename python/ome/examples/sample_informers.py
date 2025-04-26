"""
Sample code demonstrating how to use the OME informers.
"""

import threading
import time
from typing import Any, List, Optional

from ome import OMEClient, V1beta1ClusterBaseModel, V1beta1InferenceService
from ome.informers.factory import SharedInformerFactory
from ome.informers.shared_informer import ResourceEventHandler
from ome.utils.logger import logger


class InferenceServiceEventHandler(ResourceEventHandler):
    """
    Handler for InferenceService events.
    """

    def on_add(self, obj: V1beta1InferenceService) -> None:
        """
        Called when an InferenceService is added.

        Args:
            obj: The added InferenceService
        """
        logger.info(
            f"Added InferenceService: {obj.metadata.namespace}/{obj.metadata.name}"
        )

    def on_update(
        self, old_obj: V1beta1InferenceService, new_obj: V1beta1InferenceService
    ) -> None:
        """
        Called when an InferenceService is updated.

        Args:
            old_obj: The old InferenceService
            new_obj: The updated InferenceService
        """
        logger.info(
            f"Updated InferenceService: {new_obj.metadata.namespace}/{new_obj.metadata.name}"
        )

    def on_delete(self, obj: V1beta1InferenceService) -> None:
        """
        Called when an InferenceService is deleted.

        Args:
            obj: The deleted InferenceService
        """
        logger.info(
            f"Deleted InferenceService: {obj.metadata.namespace}/{obj.metadata.name}"
        )


class ClusterBaseModelEventHandler(ResourceEventHandler):
    """
    Handler for ClusterBaseModel events.
    """

    def on_add(self, obj: V1beta1ClusterBaseModel) -> None:
        """
        Called when a ClusterBaseModel is added.

        Args:
            obj: The added ClusterBaseModel
        """
        logger.info(f"Added ClusterBaseModel: {obj.metadata.name}")

    def on_update(
        self, old_obj: V1beta1ClusterBaseModel, new_obj: V1beta1ClusterBaseModel
    ) -> None:
        """
        Called when a ClusterBaseModel is updated.

        Args:
            old_obj: The old ClusterBaseModel
            new_obj: The updated ClusterBaseModel
        """
        logger.info(f"Updated ClusterBaseModel: {new_obj.metadata.name}")

    def on_delete(self, obj: V1beta1ClusterBaseModel) -> None:
        """
        Called when a ClusterBaseModel is deleted.

        Args:
            obj: The deleted ClusterBaseModel
        """
        logger.info(f"Deleted ClusterBaseModel: {obj.metadata.name}")


def main():
    """
    Main function demonstrating informers usage.
    """
    # Create a client
    client = OMEClient()

    # Create the shared informer factory with 30-second resync period
    factory = SharedInformerFactory(client, default_resync=30.0)

    # Get informers for specific resources
    inference_informer = factory.ome().v1beta1().inference_services()
    cluster_base_model_informer = factory.ome().v1beta1().cluster_base_models()

    # Add event handlers
    inference_informer.informer().add_event_handler(InferenceServiceEventHandler())
    cluster_base_model_informer.informer().add_event_handler(
        ClusterBaseModelEventHandler()
    )

    # Create a stop event
    stop_event = threading.Event()

    try:
        logger.info("Starting informers...")

        # Start the informer factory
        factory.start(stop_event)

        # Wait for all caches to sync
        logger.info("Waiting for informer caches to sync...")
        sync_result = factory.wait_for_cache_sync(stop_event)

        for informer_type, synced in sync_result.items():
            logger.info(f"Informer for {informer_type.__name__} synced: {synced}")

        # After sync, you can use the listers
        inference_lister = inference_informer.lister()
        cluster_base_model_lister = cluster_base_model_informer.lister()

        # List all InferenceServices in namespace "default"
        services = inference_lister.inference_services("default").list()
        logger.info(f"Found {len(services)} InferenceServices in namespace 'default'")

        # List all ClusterBaseModels
        cluster_models = cluster_base_model_lister.list()
        logger.info(f"Found {len(cluster_models)} ClusterBaseModels")

        # Keep the informers running
        logger.info("Informers are running. Press Ctrl+C to stop.")
        while True:
            time.sleep(1)

    except KeyboardInterrupt:
        logger.info("Shutting down...")
    finally:
        # Signal all informers to stop
        stop_event.set()
        # Wait for informers to shut down
        factory.shutdown()
        logger.info("Informers shut down")


if __name__ == "__main__":
    main()
