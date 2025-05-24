"""
ClusterBaseModel lister implementation.
"""

from typing import List, Optional

from ome import V1beta1ClusterBaseModel
from ome.informers.cache import Indexer
from ome.listers.base_lister import BaseLister
from ome.utils.logger import logger


class ClusterBaseModelLister(BaseLister[V1beta1ClusterBaseModel]):
    """
    ClusterBaseModelLister is able to list and get ClusterBaseModel resources.
    """

    def __init__(self, indexer: Indexer):
        """
        Initialize a new ClusterBaseModelLister.

        Args:
            indexer: The indexer to use
        """
        super().__init__(indexer)

    def list(self) -> List[V1beta1ClusterBaseModel]:
        """
        List all ClusterBaseModels.

        Returns:
            List of ClusterBaseModels
        """
        return super().list()

    def get(self, name: str) -> Optional[V1beta1ClusterBaseModel]:
        """
        Get a specific ClusterBaseModel by name.

        Args:
            name: Name of the ClusterBaseModel

        Returns:
            The ClusterBaseModel or None if not found
        """
        return super().get(name)
