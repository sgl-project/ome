
# OME Python SDK

The guide shows how to generate the openapi model and swagger.json file from OME types using `openapi-gen` and generate Python SDK Client for the Python object models using `openapi-codegen`.

## Generate openapi spec and swagger file.

From root folder, you can `make generate` or execute the below script directly to generate openapi spec and swagger file.

```
./hack/update-openapigen.sh
```
After executing, the `openapi_generated.go` and `swagger.json` are generated and stored under `pkg/openapi/`.

## Generate Python SDK

From root folder, execute the script `/hack/python-sdk/client-gen.sh` to install openapi-codegen and generate Python SDK.

```
./hack/python-sdk/client-gen.sh
```
After the script execution, the Python SDK is generated in the `python/ome` directory. Some files such as [README](../../python/ome/README.md) and documents need to be merged manually after the script execution.

