"""
Base lister implementation that provides generic cache listing capabilities.
"""

from typing import (
    Any,
    Callable,
    Dict,
    Generic,
    List,
    Optional,
    Set,
    Tuple,
    TypeVar,
    Union,
)

from ome.informers.cache import Indexer
from ome.utils.logger import logger

T = TypeVar("T")


class BaseLister(Generic[T]):
    """
    BaseLister provides a generic base class for all listers.
    """

    def __init__(self, indexer: Indexer):
        """
        Initialize a new BaseLister.

        Args:
            indexer: The indexer to use for listing
        """
        self._indexer = indexer

    def list(self, namespace: str = None) -> List[T]:
        """
        List all objects in the store, optionally filtered by namespace.

        Args:
            namespace: Optional namespace to filter by

        Returns:
            List of matching objects
        """
        if namespace and hasattr(self._indexer, "index"):
            # Use namespace index if available
            try:
                keys = self._indexer.index_keys("namespace", namespace)
                result = []
                for key in keys:
                    obj = self._indexer.get_by_key(key)
                    if obj:
                        result.append(obj)
                return result
            except Exception as e:
                logger.warning(f"Error using namespace index: {e}")
                # Fall back to filtering the full list

        # List all objects and filter by namespace if provided
        all_objects = self._indexer.list()

        if not namespace:
            return all_objects

        # Filter by namespace manually
        return [
            obj
            for obj in all_objects
            if hasattr(obj, "metadata")
            and getattr(obj.metadata, "namespace", None) == namespace
        ]

    def get(self, name: str, namespace: str = None) -> Optional[T]:
        """
        Get an object by name and namespace.

        Args:
            name: Name of the object
            namespace: Namespace of the object (for namespaced resources)

        Returns:
            The matching object or None if not found
        """
        if namespace:
            key = f"{namespace}/{name}"
        else:
            key = name

        return self._indexer.get_by_key(key)


class NamespaceLister(BaseLister[T]):
    """
    NamespaceLister provides methods to list resources in a specific namespace.
    """

    def __init__(self, indexer: Indexer, namespace: str):
        """
        Initialize a new NamespaceLister.

        Args:
            indexer: The indexer to use
            namespace: The namespace to restrict listing to
        """
        super().__init__(indexer)
        self._namespace = namespace

    def list(self) -> List[T]:
        """
        List all objects in the namespace.

        Returns:
            List of objects in the namespace
        """
        return super().list(namespace=self._namespace)

    def get(self, name: str) -> Optional[T]:
        """
        Get an object by name in this namespace.

        Args:
            name: Name of the object

        Returns:
            The matching object or None if not found
        """
        return super().get(name=name, namespace=self._namespace)
