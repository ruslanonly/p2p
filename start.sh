#!/bin/bash
set -euxo pipefail

echo "📦 Запуск threats"
/app/threats &

echo "📦 Запуск agent"
/app/agent &

wait