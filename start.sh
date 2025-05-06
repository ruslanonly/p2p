#!/bin/bash

# Запуск agent и pcap параллельно
/app/threats &
/app/agent &

# Ожидание обоих процессов
wait
