#!/bin/bash

set -euo pipefail

./test/create-test-cluster.sh
./test/build-image.sh
