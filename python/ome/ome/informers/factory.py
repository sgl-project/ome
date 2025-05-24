"""
SharedInformerFactory provides shared informers for resources in all known API groups.
"""

import threading
import time
from typing import Any, Callable, Dict, Generic, List, Optional, Set, Type, TypeVar

from kubernetes.client import ApiClient

from ome.api.ome_client import OMEClient
from ome.informers.internal_interfaces import NewInformerFunc
from ome.informers.internal_interfaces import SharedInformerFactory as InformerFactory
from ome.informers.shared_informer import SharedIndexInformer
from ome.utils.logger import logger


class SharedInformerFactory(InformerFactory):
    """
    SharedInformerFactory provides shared informers for resources in all API groups.

    It is typically used like this:

        client = OmeClient(...)
        factory = SharedInformerFactory(client, 30.0)  # 30 second resync period
        inference_informer = factory.ome().v1beta1().inference_services()
        inference_informer.add_event_handler(my_handler)

        # Start all informers
        stop_ch = threading.Event()
        factory.start(stop_ch)

        # Wait for initial sync
        factory.wait_for_cache_sync(stop_ch)

        # Use the informer...

        # When done
        stop_ch.set()  # Signal informers to stop
    """

    def __init__(
        self, client: OMEClient, default_resync: float = 60.0, namespace: str = None
    ):
        """
        Initialize a new SharedInformerFactory.

        Args:
            client: The OmeClient to use
            default_resync: Default resync period in seconds
            namespace: Optional namespace to restrict informers to
        """
        self._client = client
        self._namespace = namespace or ""
        self._default_resync = default_resync
        self._lock = threading.RLock()
        self._informers = {}  # type: Dict[Type, SharedIndexInformer]
        self._started_informers = {}  # type: Dict[Type, bool]
        self._wait_group = set()  # Set of thread events to wait for
        self._shutting_down = False
        self._transform_func = None

    def start(self, stop_ch) -> None:
        """
        Start initializes all requested informers.

        Args:
            stop_ch: Event that will be set when informers should stop
        """
        with self._lock:
            if self._shutting_down:
                return

            for obj_type, informer in self._informers.items():
                if not self._started_informers.get(obj_type, False):
                    thread_event = threading.Event()
                    self._wait_group.add(thread_event)

                    # Start the informer in its own thread
                    def run_informer(informer_inst, thread_stop):
                        try:
                            informer_inst.run(stop_ch)
                            # Wait until we're asked to stop
                            stop_ch.wait()
                        finally:
                            thread_stop.set()

                    threading.Thread(
                        target=run_informer, args=(informer, thread_event), daemon=True
                    ).start()

                    self._started_informers[obj_type] = True

    def shutdown(self) -> None:
        """
        Shutdown marks the factory as shutting down.

        After this is called, no more informers can be started.
        This will block until all informer goroutines have terminated.
        """
        with self._lock:
            self._shutting_down = True

        # Wait for all threads to finish
        for event in self._wait_group:
            event.wait()

    def wait_for_cache_sync(self, stop_ch) -> Dict[Type, bool]:
        """
        WaitForCacheSync blocks until all started informers' caches were synced
        or the stop channel gets closed.

        Args:
            stop_ch: Event that will be set when waiting should stop

        Returns:
            Dictionary mapping types to sync status
        """
        # Get started informers
        informers = {}
        with self._lock:
            for informer_type, informer in self._informers.items():
                if self._started_informers.get(informer_type, False):
                    informers[informer_type] = informer

        # Wait for all to sync or stop
        res = {}
        for informer_type, informer in informers.items():
            synced = False
            while not synced and not stop_ch.is_set():
                synced = informer.has_synced()
                if not synced:
                    time.sleep(0.1)  # Short sleep before checking again

            res[informer_type] = synced

        return res

    def informer_for(
        self, obj_type: Type, new_func: NewInformerFunc
    ) -> SharedIndexInformer:
        """
        Get a SharedIndexInformer for the specified type.

        Args:
            obj_type: Type of objects the informer should handle
            new_func: Function to create a new informer

        Returns:
            A SharedIndexInformer for the specified type
        """
        with self._lock:
            informer = self._informers.get(obj_type)
            if informer:
                return informer

            # Create new informer
            informer = new_func(self._client, self._default_resync)
            if self._transform_func:
                informer.set_transform_func(self._transform_func)

            self._informers[obj_type] = informer
            return informer

    def ome(self):
        """
        Get the OME group of informers.

        Returns:
            OmeInformer interface
        """
        from ome.informers.ome import OMEInformer

        return OMEInformer(self, self._namespace)
