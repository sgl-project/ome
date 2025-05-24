"""
OME informer implementation.
"""

from ome.informers.internal_interfaces import (
    SharedInformerFactory,
    TweakListOptionsFunc,
)


class OMEInformer:
    """
    Provides access to the OME informers.
    """

    def __init__(
        self,
        factory: SharedInformerFactory,
        namespace: str = "",
        tweak_list_options: TweakListOptionsFunc = None,
    ):
        """
        Initialize a new OmeInformer.

        Args:
            factory: The shared informer factory
            namespace: Optional namespace to restrict informers to
            tweak_list_options: Optional function to tweak list options
        """
        self._factory = factory
        self._namespace = namespace
        self._tweak_list_options = tweak_list_options

    def v1beta1(self):
        """
        Get the v1beta1 version of the OME informers.

        Returns:
            V1beta1Interface
        """
        from ome.informers.ome.v1beta1 import V1beta1Interface

        return V1beta1Interface(
            self._factory, self._namespace, self._tweak_list_options
        )
