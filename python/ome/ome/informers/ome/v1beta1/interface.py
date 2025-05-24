"""
V1beta1 interface for OME informers.
"""

from ome.informers.internal_interfaces import (
    SharedInformerFactory,
    TweakListOptionsFunc,
)


class V1beta1Interface:
    """
    Interface provides access to all the informers in this group version.
    """

    def __init__(
        self,
        factory: SharedInformerFactory,
        namespace: str = "",
        tweak_list_options: TweakListOptionsFunc = None,
    ):
        """
        Initialize a new V1beta1Interface.

        Args:
            factory: The shared informer factory
            namespace: Optional namespace to restrict informers to
            tweak_list_options: Optional function to tweak list options
        """
        self._factory = factory
        self._namespace = namespace
        self._tweak_list_options = tweak_list_options

    def base_models(self):
        """
        Get the BaseModel informer.

        Returns:
            BaseModelInformer
        """
        from ome.informers.ome.v1beta1.base_model import BaseModelInformer

        return BaseModelInformer(
            self._factory, self._namespace, self._tweak_list_options
        )

    def cluster_base_models(self):
        """
        Get the ClusterBaseModel informer.

        Returns:
            ClusterBaseModelInformer
        """
        from ome.informers.ome.v1beta1.cluster_base_model import (
            ClusterBaseModelInformer,
        )

        return ClusterBaseModelInformer(self._factory, self._tweak_list_options)

    def inference_services(self):
        """
        Get the InferenceService informer.

        Returns:
            InferenceServiceInformer
        """
        from ome.informers.ome.v1beta1.inference_service import InferenceServiceInformer

        return InferenceServiceInformer(
            self._factory, self._namespace, self._tweak_list_options
        )

    def projects(self):
        """
        Get the Project informer.

        Returns:
            ProjectInformer
        """
        from ome.informers.ome.v1beta1.project import ProjectInformer

        return ProjectInformer(self._factory, self._tweak_list_options)

    def organizations(self):
        """
        Get the Organization informer.

        Returns:
            OrganizationInformer
        """
        from ome.informers.ome.v1beta1.organization import OrganizationInformer

        return OrganizationInformer(self._factory, self._tweak_list_options)

    def service_accounts(self):
        """
        Get the ServiceAccount informer.

        Returns:
            ServiceAccountInformer
        """
        from ome.informers.ome.v1beta1.service_account import ServiceAccountInformer

        return ServiceAccountInformer(
            self._factory, self._namespace, self._tweak_list_options
        )

    def inference_graphs(self):
        """
        Get the InferenceGraph informer.

        Returns:
            InferenceGraphInformer
        """
        from ome.informers.ome.v1beta1.inference_graph import InferenceGraphInformer

        return InferenceGraphInformer(
            self._factory, self._namespace, self._tweak_list_options
        )

    def serving_runtimes(self):
        """
        Get the ServingRuntime informer.

        Returns:
            ServingRuntimeInformer
        """
        from ome.informers.ome.v1beta1.serving_runtime import ServingRuntimeInformer

        return ServingRuntimeInformer(
            self._factory, self._namespace, self._tweak_list_options
        )

    def cluster_serving_runtimes(self):
        """
        Get the ClusterServingRuntime informer.

        Returns:
            ClusterServingRuntimeInformer
        """
        from ome.informers.ome.v1beta1.cluster_serving_runtime import (
            ClusterServingRuntimeInformer,
        )

        return ClusterServingRuntimeInformer(self._factory, self._tweak_list_options)

    def training_jobs(self):
        """
        Get the TrainingJob informer.

        Returns:
            TrainingJobInformer
        """
        from ome.informers.ome.v1beta1.training_job import TrainingJobInformer

        return TrainingJobInformer(
            self._factory, self._namespace, self._tweak_list_options
        )

    def training_runtimes(self):
        """
        Get the TrainingRuntime informer.

        Returns:
            TrainingRuntimeInformer
        """
        from ome.informers.ome.v1beta1.training_runtime import TrainingRuntimeInformer

        return TrainingRuntimeInformer(
            self._factory, self._namespace, self._tweak_list_options
        )

    def cluster_training_runtimes(self):
        """
        Get the ClusterTrainingRuntime informer.

        Returns:
            ClusterTrainingRuntimeInformer
        """
        from ome.informers.ome.v1beta1.cluster_training_runtime import (
            ClusterTrainingRuntimeInformer,
        )

        return ClusterTrainingRuntimeInformer(self._factory, self._tweak_list_options)

    def fine_tuned_weights(self):
        """
        Get the FineTunedWeight informer.

        Returns:
            FineTunedWeightInformer
        """
        from ome.informers.ome.v1beta1.fine_tuned_weight import FineTunedWeightInformer

        return FineTunedWeightInformer(
            self._factory, self._namespace, self._tweak_list_options
        )
