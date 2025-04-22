import time

from kubernetes import client
from kubernetes import watch as k8s_watch
from tabulate import tabulate

from ome.constants import constants
from ome.utils import utils
from ome.utils.logger import logger


def inference_service_watch(
    name=None, namespace=None, timeout_seconds=600, generation=0
):
    """Watch the created or patched InferenceService in the specified namespace"""

    if namespace is None:
        namespace = utils.get_default_target_namespace()

    headers = ["NAME", "READY", "PREV", "LATEST", "URL"]
    table_fmt = "plain"

    stream = k8s_watch.Watch().stream(
        client.CustomObjectsApi().list_namespaced_custom_object,
        constants.OME_GROUP,
        constants.OME_V1BETA1_VERSION,
        namespace,
        constants.OME_PLURAL_INFERENCESERVICE,
        timeout_seconds=timeout_seconds,
    )

    for event in stream:
        isvc = event["object"]
        isvc_name = isvc["metadata"]["name"]
        if name and name != isvc_name:
            continue
        else:
            status = "Unknown"
            if isvc.get("status", ""):
                url = isvc["status"].get("url", "")
                traffic = (
                    isvc["status"]
                    .get("components", {})
                    .get("predictor", {})
                    .get("traffic", [])
                )
                traffic_percent = 100
                if constants.OBSERVED_GENERATION in isvc["status"]:
                    observed_generation = isvc["status"][constants.OBSERVED_GENERATION]
                    for t in traffic:
                        if t["latestRevision"]:
                            traffic_percent = t["percent"]

                    if generation != 0 and observed_generation != generation:
                        continue
                    for condition in isvc["status"].get("conditions", {}):
                        if condition.get("type", "") == "Ready":
                            status = condition.get("status", "Unknown")
                    logger.info(
                        tabulate(
                            [
                                [
                                    isvc_name,
                                    status,
                                    100 - traffic_percent,
                                    traffic_percent,
                                    url,
                                ]
                            ],
                            headers=headers,
                            tablefmt=table_fmt,
                        )
                    )
                    if status == "True":
                        break

            else:
                logger.info(
                    tabulate(
                        [[isvc_name, status, "", "", ""]],
                        headers=headers,
                        tablefmt=table_fmt,
                    )
                )
                # Sleep 2 to avoid status section is not generated within a very short time.
                time.sleep(2)
                continue
