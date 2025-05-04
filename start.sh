#!/bin/bash

# Запуск agent и pcap параллельно
/app/traffic &
/app/agent &

# Ожидание обоих процессов
wait
