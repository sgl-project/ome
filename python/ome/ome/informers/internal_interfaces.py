"""
Internal interfaces for the informer factory.
"""

from typing import Any, Callable, Dict, Optional, TypeVar

from kubernetes.client import ApiClient
from kubernetes.client.rest import ApiException

from ome.utils.logger import logger

# Type for tweaking list options
TweakListOptionsFunc = Callable[[Dict[str, Any]], None]
T = TypeVar("T")


class NewInformerFunc:
    """Function type for creating a new informer."""

    def __call__(self, client: ApiClient, resync_period: float):
        """
        Takes an ApiClient and resync period to return a SharedIndexInformer.

        Args:
            client: The API client to use
            resync_period: The resync period in seconds

        Returns:
            A SharedIndexInformer
        """
        pass


class SharedInformerFactory:
    """
    SharedInformerFactory is a small interface to allow for adding an informer without an import cycle.
    """

    def start(self, stop_ch):
        """
        Start all informers.

        Args:
            stop_ch: Channel to signal when to stop
        """
        pass

    def informer_for(self, obj_type, new_func):
        """
        Get an informer for a specific object type.

        Args:
            obj_type: The object type
            new_func: Function to create a new informer

        Returns:
            A SharedIndexInformer for the specified type
        """
        pass
