package capacityreservation

import (
	"context"
	"net/http"

	"github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/capacityreservation/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// We use a separate logger name here, e.g. "capacityreservation-webhook"
var log = logf.Log.WithName("capacityreservation-webhook")

// +kubebuilder:webhook:verbs=create;update,path=/validate-ome-io-v1beta1-clustercapacityreservation,mutating=false,failurePolicy=fail,groups=ome.io,resources=clustercapacityreservations,versions=v1beta1,name=clustercapacityreservation.ome-webhook-server.validator

type CapacityReservationValidator struct {
	Client  client.Client
	Decoder admission.Decoder
}

func (v *CapacityReservationValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	capacityReservation := &v1beta1.ClusterCapacityReservation{}

	err := v.Decoder.DecodeRaw(req.Object, capacityReservation)
	if err != nil {
		log.Error(err, "Failed to decode cluster capacity reservation request")
		return admission.Errored(http.StatusBadRequest, err)
	}

	log.Info("Processing capacity reservation request", "name", capacityReservation.Name, "compartmentID", capacityReservation.Spec.CompartmentID)

	// Deny the request if no resource groups exist.
	if len(capacityReservation.Spec.ResourceGroups) == 0 {
		log.Info("Rejecting capacity reservation: no resource groups specified")
		return admission.Denied("No resource groups specified in ClusterCapacityReservation")
	}

	// Get available resources from the cluster
	availableResources, err := utils.GetClusterAvailableResource()
	if err != nil {
		log.Error(err, "Failed to get cluster available resources")
		return admission.Errored(http.StatusInternalServerError, err)
	}
	log.Info("Retrieved available cluster resources", "resources", availableResources)

	// Convert resource groups to map format
	requestedResources := utils.ConvertResourceGroupsToMap(capacityReservation.Spec.ResourceGroups)
	log.Info("Converted requested resources", "resources", requestedResources)

	// Get existing capacity reservations
	existingCapacityReservations := &v1beta1.ClusterCapacityReservationList{}
	if err := v.Client.List(ctx, existingCapacityReservations); err != nil {
		log.Error(err, "Failed to list existing capacity reservations")
		return admission.Errored(http.StatusInternalServerError, err)
	}
	log.Info("Retrieved existing capacity reservations", "count", len(existingCapacityReservations.Items))

	// Get total capacities from existing reservations
	existingCapacities := utils.GetTotalCapacitiesFromCapacityReservationList(*existingCapacityReservations)
	log.Info("Calculated existing capacities", "capacities", existingCapacities)

	// Check if requested resources are sufficient
	if !utils.IsResourceSufficient(availableResources, existingCapacities, requestedResources) {
		log.Info("Insufficient resources for capacity reservation",
			"available", availableResources,
			"existing", existingCapacities,
			"requested", requestedResources)
		return admission.Denied("Insufficient resources in the cluster for the requested capacity reservation")
	}

	log.Info("Capacity reservation validation passed", "name", capacityReservation.Name)
	return admission.Allowed("ClusterCapacityReservation is valid")
}
