"""
Cache implementations for the informer framework.
"""

import threading
import time
from typing import Any, Callable, Dict, Generic, List, Optional, Set, TypeVar

from ome.utils.logger import logger

T = TypeVar("T")
KeyFunc = Callable[[T], str]
ResourceVersion = str


class Store(Generic[T]):
    """
    Store is a generic object storage and processing interface.
    """

    def add(self, obj: T) -> None:
        """Add an object to the store."""
        pass

    def update(self, obj: T) -> None:
        """Update an object in the store."""
        pass

    def delete(self, obj: T) -> None:
        """Delete an object from the store."""
        pass

    def list(self) -> List[T]:
        """List all objects in the store."""
        pass

    def list_keys(self) -> List[str]:
        """List all keys in the store."""
        pass

    def get(self, obj: T) -> Optional[T]:
        """Get an object from the store."""
        pass

    def get_by_key(self, key: str) -> Optional[T]:
        """Get an object by key from the store."""
        pass

    def replace(self, list_obj: List[T], resource_version: str) -> None:
        """Replace the store contents with the given list."""
        pass


class Indexer(Store[T]):
    """
    Indexer extends Store with multiple indices and restricts each
    accumulator to simply hold the current object (and be empty after Delete).
    """

    def index(self, index_name: str, obj: T) -> List[str]:
        """
        Retrieve a list of the keys that match the given object on the named index.

        Args:
            index_name: Name of the index to check
            obj: Object to get matches for

        Returns:
            List of keys that match
        """
        pass

    def index_keys(self, index_name: str, index_key: str) -> List[str]:
        """
        Return the set of keys that match on the named index with the given value.

        Args:
            index_name: Name of the index to check
            index_key: Value to check

        Returns:
            List of keys that match
        """
        pass

    def list_index_func_values(self, index_name: str) -> List[str]:
        """
        List all the values available in the named index.

        Args:
            index_name: Name of the index

        Returns:
            List of values in the index
        """
        pass

    def get_indexers(self) -> Dict[str, Callable[[T], List[str]]]:
        """
        Return the indexers registered with this store.

        Returns:
            Dictionary mapping names to index functions
        """
        pass

    def add_indexers(self, new_indexers: Dict[str, Callable[[T], List[str]]]) -> None:
        """
        Add indexers to the indexer.

        Args:
            new_indexers: Dictionary mapping names to index functions
        """
        pass


class ThreadSafeStore(Generic[T]):
    """Thread safe storage interface."""

    def add(self, key: str, obj: T) -> None:
        """Add an object to the store with the given key."""
        pass

    def update(self, key: str, obj: T) -> None:
        """Update an object in the store with the given key."""
        pass

    def delete(self, key: str) -> None:
        """Delete an object from the store with the given key."""
        pass

    def get(self, key: str) -> Optional[T]:
        """Get an object from the store by key."""
        pass

    def list(self) -> Dict[str, T]:
        """List all objects in the store."""
        pass

    def list_keys(self) -> List[str]:
        """List all keys in the store."""
        pass

    def replace(self, items: Dict[str, T], resource_version: str) -> None:
        """Replace the contents of the store with the given items."""
        pass

    def resync(self) -> None:
        """Resync the store contents."""
        pass


class ThreadSafeMap(ThreadSafeStore[T]):
    """
    ThreadSafeMap implements ThreadSafeStore with a thread-safe map.
    """

    def __init__(self):
        self._items = {}  # type: Dict[str, T]
        self._lock = threading.RLock()
        self._resource_version = ""

    def add(self, key: str, obj: T) -> None:
        """
        Add adds the given object to the store.

        Args:
            key: The key of the object
            obj: The object to add
        """
        with self._lock:
            self._items[key] = obj

    def update(self, key: str, obj: T) -> None:
        """
        Update updates the given object in the store.

        Args:
            key: The key of the object
            obj: The updated object
        """
        with self._lock:
            self._items[key] = obj

    def delete(self, key: str) -> None:
        """
        Delete removes the object from the store by key.

        Args:
            key: The key of the object
        """
        with self._lock:
            if key in self._items:
                del self._items[key]

    def get(self, key: str) -> Optional[T]:
        """
        Get returns the object by key.

        Args:
            key: The key of the object

        Returns:
            The requested object or None if not found
        """
        with self._lock:
            return self._items.get(key)

    def list(self) -> Dict[str, T]:
        """
        List returns a copy of the objects in the store.

        Returns:
            Dictionary of all objects
        """
        with self._lock:
            return dict(self._items)

    def list_keys(self) -> List[str]:
        """
        ListKeys returns a list of all the keys of the objects in the store.

        Returns:
            List of keys
        """
        with self._lock:
            return list(self._items.keys())

    def replace(self, items: Dict[str, T], resource_version: str) -> None:
        """
        Replace will delete the contents of the store, using instead the
        given map. Replace takes care of locking the store.

        Args:
            items: Map of items to use instead
            resource_version: Resource version to set
        """
        with self._lock:
            self._items = dict(items)
            self._resource_version = resource_version

    def resync(self) -> None:
        """
        Resync is meaningless for an in-memory cache, but here to satisfy the interface.
        """
        pass
