"""
SharedInformer provides shared caches of resources for all clients.
"""

import threading
import time
from typing import Any, Callable, Dict, Generic, List, Optional, Set, Type, TypeVar

from kubernetes.client import ApiClient
from kubernetes.watch import watch

from ome.informers.cache import Indexer, Store, ThreadSafeMap
from ome.utils.logger import logger

T = TypeVar("T")
WatchEventType = str  # "ADDED", "MODIFIED", "DELETED", "BOOKMARK", "ERROR"


class ProcessFunc(Generic[T]):
    """Function type for processing watch events."""

    def __call__(self, obj: T) -> None:
        """Process a watch event object."""
        pass


class SharedIndexInformer:
    """
    SharedIndexInformer provides add and get Indexers ability based on SharedInformer.
    It is thread-safe.
    """

    def add_event_handler(self, handler):
        """
        Add an event handler to the shared informer.

        Args:
            handler: An event handler with OnAdd, OnUpdate and OnDelete methods
        """
        pass

    def add_event_handler_with_resync_period(self, handler, resync_period: float):
        """
        Add an event handler with a specific resync period.

        Args:
            handler: An event handler with OnAdd, OnUpdate and OnDelete methods
            resync_period: Resync period in seconds
        """
        pass

    def get_indexer(self) -> Indexer:
        """
        Get the indexer for this informer.

        Returns:
            Indexer for this informer
        """
        pass

    def add_indexers(self, indexers: Dict[str, Callable[[T], List[str]]]):
        """
        Add indexers to this informer.

        Args:
            indexers: Dictionary mapping names to indexer functions
        """
        pass

    def has_synced(self) -> bool:
        """
        Check if this informer has completed an initial full synchronization.

        Returns:
            True if the informer has synced
        """
        pass

    def last_sync_resource_version(self) -> str:
        """
        Get the last known resource version for this informer.

        Returns:
            Resource version string
        """
        pass

    def run(self, stop_ch):
        """
        Run the informer until the given stop channel is closed.

        Args:
            stop_ch: Channel that will be closed when informer should stop
        """
        pass

    def set_transform_func(self, transform_func):
        """
        Set a transform function for this informer.

        Args:
            transform_func: Function that transforms objects before storing
        """
        pass


class SharedInformer(SharedIndexInformer):
    """
    SharedInformer provides eventual consistency and incremental object updates.
    """

    pass


class ResourceEventHandler(Generic[T]):
    """
    ResourceEventHandler can handle notifications for events that happen to a resource.
    """

    def on_add(self, obj: T) -> None:
        """
        OnAdd is called when an object is added.

        Args:
            obj: The added object
        """
        pass

    def on_update(self, old_obj: T, new_obj: T) -> None:
        """
        OnUpdate is called when an object is modified.

        Args:
            old_obj: The old object
            new_obj: The new object
        """
        pass

    def on_delete(self, obj: T) -> None:
        """
        OnDelete is called when an object is deleted.

        Args:
            obj: The deleted object
        """
        pass


class SharedInformerImpl(Generic[T], SharedInformer):
    """Implementation of the SharedInformer interface."""

    def __init__(
        self,
        client: ApiClient,
        resource_type: Type[T],
        list_func,
        watch_func,
        namespace: str = None,
        field_selector: str = None,
        label_selector: str = None,
        resync_period: float = 60.0,
    ):
        """
        Initialize a new SharedInformerImpl.

        Args:
            client: API client
            resource_type: Type of resource this informer handles
            list_func: Function to list resources
            watch_func: Function to watch resources
            namespace: Namespace to restrict to (if applicable)
            field_selector: Field selector to filter resources
            label_selector: Label selector to filter resources
            resync_period: How often to resync the informer in seconds
        """
        self._client = client
        self._resource_type = resource_type
        self._list_func = list_func
        self._watch_func = watch_func
        self._namespace = namespace
        self._field_selector = field_selector
        self._label_selector = label_selector
        self._resync_period = resync_period
        self._started = False
        self._stopped = False
        self._store = ThreadSafeMap()
        self._controller = None
        self._event_handlers = []
        self._transform_func = None
        self._last_sync_resource_version = ""
        self._has_synced_lock = threading.RLock()
        self._has_synced_condition = threading.Condition(self._has_synced_lock)
        self._has_synced_value = False
        self._watcher = None

    def add_event_handler(self, handler: ResourceEventHandler) -> None:
        """
        Add an event handler to the shared informer.

        Args:
            handler: An event handler for resource events
        """
        if self._stopped:
            logger.warning("Informer has stopped, ignoring event handler addition")
            return

        self._event_handlers.append((handler, self._resync_period))

        # If already started, schedule resource notifications
        if self._started:
            # Simulate an add for all existing items
            for key, item in self._store.list().items():
                handler.on_add(item)

    def add_event_handler_with_resync_period(
        self, handler: ResourceEventHandler, resync_period: float
    ) -> None:
        """
        Add an event handler with a custom resync period.

        Args:
            handler: An event handler for resource events
            resync_period: Custom resync period in seconds
        """
        if self._stopped:
            logger.warning("Informer has stopped, ignoring event handler addition")
            return

        self._event_handlers.append((handler, resync_period))

        # If already started, schedule resource notifications
        if self._started:
            # Simulate an add for all existing items
            for key, item in self._store.list().items():
                handler.on_add(item)

    def get_indexer(self) -> Indexer:
        """
        Get the indexer for this informer.

        Returns:
            Indexer for this informer
        """
        # Not implemented in the base class, will be extended by SharedIndexInformerImpl
        return None

    def add_indexers(self, indexers: Dict[str, Callable[[T], List[str]]]) -> None:
        """
        Add indexers to this informer.

        Args:
            indexers: Dictionary mapping names to indexer functions
        """
        # Not implemented in the base class, will be extended by SharedIndexInformerImpl
        pass

    def has_synced(self) -> bool:
        """
        Check if this informer has completed an initial full synchronization.

        Returns:
            True if the informer has synced
        """
        with self._has_synced_lock:
            return self._has_synced_value

    def last_sync_resource_version(self) -> str:
        """
        Get the last known resource version for this informer.

        Returns:
            Resource version string
        """
        return self._last_sync_resource_version

    def run(self, stop_ch) -> None:
        """
        Run the informer until the given stop channel is closed.

        Args:
            stop_ch: Channel that will be closed when informer should stop
        """
        if self._started:
            logger.warning("Informer already started, ignoring run call")
            return

        self._started = True
        self._stopped = False

        # Start the controller in a background thread
        self._run_controller(stop_ch)

    def _run_controller(self, stop_ch) -> None:
        """
        Run the controller in a background thread.

        Args:
            stop_ch: Channel that will be closed when controller should stop
        """

        def controller_thread():
            try:
                while not self._stopped:
                    try:
                        # First, do a full list
                        self._full_list_and_watch(stop_ch)
                    except Exception as e:
                        logger.exception(f"Error in informer controller: {e}")

                    # If we got here, something failed with the watch
                    # Wait a bit before retrying to avoid hammering the API
                    if not self._stopped:
                        time.sleep(1)
            finally:
                self._stopped = True
                logger.info(
                    f"Informer controller for {self._resource_type.__name__} stopped"
                )

        # Start the controller thread
        threading.Thread(target=controller_thread, daemon=True).start()

    def _full_list_and_watch(self, stop_ch) -> None:
        """
        Perform a full list and then watch for changes.

        Args:
            stop_ch: Channel that will be closed when controller should stop
        """
        # List all resources
        list_params = {}
        if self._namespace is not None:
            list_params["namespace"] = self._namespace
        if self._field_selector is not None:
            list_params["field_selector"] = self._field_selector
        if self._label_selector is not None:
            list_params["label_selector"] = self._label_selector

        try:
            result = self._list_func(**list_params)

            # Process list results
            items = getattr(result, "items", [])
            self._resource_version = getattr(result, "metadata", {}).get(
                "resourceVersion", ""
            )
            self._last_sync_resource_version = self._resource_version

            # Store items
            store_items = {}
            for item in items:
                transformed_item = item
                if self._transform_func:
                    transformed_item = self._transform_func(item)
                obj_key = self._get_key(transformed_item)
                store_items[obj_key] = transformed_item

            self._store.replace(store_items, self._resource_version)

            # Mark as synced
            with self._has_synced_lock:
                self._has_synced_value = True
                self._has_synced_condition.notify_all()

            # Notify handlers of all resources
            for handler, _ in self._event_handlers:
                for item in store_items.values():
                    handler.on_add(item)

            # Now start watching
            self._watch_resources(stop_ch)

        except Exception as e:
            logger.exception(f"Error listing resources: {e}")
            # Don't mark as synced if we failed to list
            with self._has_synced_lock:
                self._has_synced_value = False

    def _watch_resources(self, stop_ch) -> None:
        """
        Watch for resource changes.

        Args:
            stop_ch: Channel that will be closed when watching should stop
        """
        watch_params = {}
        if self._namespace is not None:
            watch_params["namespace"] = self._namespace
        if self._field_selector is not None:
            watch_params["field_selector"] = self._field_selector
        if self._label_selector is not None:
            watch_params["label_selector"] = self._label_selector
        if self._resource_version:
            watch_params["resource_version"] = self._resource_version

        # Create watcher
        self._watcher = watch.Watch()

        try:
            # Start watching
            for event in self._watcher.stream(self._watch_func, **watch_params):
                # Check if we should stop
                if self._stopped:
                    break

                # Process the event
                event_type = event.get("type", "")
                obj = event.get("object")

                if not obj:
                    continue

                # Apply transform if needed
                transformed_obj = obj
                if self._transform_func:
                    transformed_obj = self._transform_func(obj)

                obj_key = self._get_key(transformed_obj)

                # Update resource version
                self._resource_version = getattr(obj, "metadata", {}).get(
                    "resourceVersion", self._resource_version
                )
                self._last_sync_resource_version = self._resource_version

                # Process based on event type
                if event_type == "ADDED":
                    old_obj = self._store.get(obj_key)
                    self._store.add(obj_key, transformed_obj)

                    # Notify handlers
                    for handler, _ in self._event_handlers:
                        handler.on_add(transformed_obj)

                elif event_type == "MODIFIED":
                    old_obj = self._store.get(obj_key)
                    self._store.update(obj_key, transformed_obj)

                    # Notify handlers
                    for handler, _ in self._event_handlers:
                        if old_obj:
                            handler.on_update(old_obj, transformed_obj)
                        else:
                            # No old object, treat as add
                            handler.on_add(transformed_obj)

                elif event_type == "DELETED":
                    old_obj = self._store.get(obj_key)
                    self._store.delete(obj_key)

                    # Notify handlers
                    if old_obj:
                        for handler, _ in self._event_handlers:
                            handler.on_delete(old_obj)

                elif event_type == "ERROR":
                    logger.error(f"Error event received: {obj}")

        except Exception as e:
            if not self._stopped:
                logger.exception(f"Error watching resources: {e}")

    def _get_key(self, obj) -> str:
        """
        Get a key for the given object.

        Args:
            obj: The object to get a key for

        Returns:
            String key for the object
        """
        # Default implementation uses namespace/name if available, otherwise fallback to string repr
        try:
            namespace = getattr(obj, "metadata", {}).get("namespace", "")
            name = getattr(obj, "metadata", {}).get("name", "")
            if namespace and name:
                return f"{namespace}/{name}"
            elif name:
                return name
        except (AttributeError, TypeError):
            pass

        # Fallback to string representation
        return str(obj)

    def set_transform_func(self, transform_func) -> None:
        """
        Set a transform function for this informer.

        Args:
            transform_func: Function that transforms objects before storing
        """
        self._transform_func = transform_func
