#!/bin/bash

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)

cd "${SCRIPT_DIR}/../.."
docker build -t localhost:5000/dns4acme .
docker push localhost:5000/dns4acme
