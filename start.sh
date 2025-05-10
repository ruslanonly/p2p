#!/bin/bash
set -euxo pipefail

/app/threats &

/app/agent &

wait