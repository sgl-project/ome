#!/bin/bash

set -e

if [ "$SKIP_LOADING_BASE_MODEL" == "true" ];
then
    echo "SERVING_RUNTIME is $SERVING_RUNTIME"
    echo "FINETUNING_STRATEGY is $FINETUNING_STRATEGY"
    echo "Skip loading the base model for finetuned serving."
else
    /serving-init serving-init -c /configs/serving-init.yaml
fi

if [ "$FINETUNE" == "true" ];
then
    /serving-ft serving-ft -c /configs/serving-ft.yaml
else
    echo "No finetuning involved."
fi
