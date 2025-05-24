"""
InferenceService lister implementation.
"""

from typing import List, Optional

from ome import V1beta1InferenceService
from ome.informers.cache import Indexer
from ome.listers.base_lister import BaseLister, NamespaceLister
from ome.utils.logger import logger


class InferenceServiceLister(BaseLister[V1beta1InferenceService]):
    """
    InferenceServiceLister is able to list and get InferenceService resources.
    """

    def __init__(self, indexer: Indexer):
        """
        Initialize a new InferenceServiceLister.

        Args:
            indexer: The indexer to use
        """
        super().__init__(indexer)

    def inference_services(self, namespace: str) -> "NamespacedInferenceServiceLister":
        """
        Return a lister for the given namespace.

        Args:
            namespace: The namespace to list in

        Returns:
            A namespaced lister
        """
        return NamespacedInferenceServiceLister(self._indexer, namespace)

    def list(self, namespace: str = None) -> List[V1beta1InferenceService]:
        """
        List all InferenceServices across all namespaces, or in the specified namespace.

        Args:
            namespace: Optional namespace to filter by

        Returns:
            List of InferenceServices
        """
        return super().list(namespace)

    def get(self, name: str, namespace: str) -> Optional[V1beta1InferenceService]:
        """
        Get a specific InferenceService by name and namespace.

        Args:
            name: Name of the InferenceService
            namespace: Namespace of the InferenceService

        Returns:
            The InferenceService or None if not found
        """
        return super().get(name, namespace)


class NamespacedInferenceServiceLister(NamespaceLister[V1beta1InferenceService]):
    """
    NamespacedInferenceServiceLister is able to list and get InferenceService resources in a specific namespace.
    """

    def __init__(self, indexer: Indexer, namespace: str):
        """
        Initialize a new NamespacedInferenceServiceLister.

        Args:
            indexer: The indexer to use
            namespace: The namespace to restrict listing to
        """
        super().__init__(indexer, namespace)

    def list(self) -> List[V1beta1InferenceService]:
        """
        List all InferenceServices in the namespace.

        Returns:
            List of InferenceServices
        """
        return super().list()

    def get(self, name: str) -> Optional[V1beta1InferenceService]:
        """
        Get a specific InferenceService by name in this namespace.

        Args:
            name: Name of the InferenceService

        Returns:
            The InferenceService or None if not found
        """
        return super().get(name)
