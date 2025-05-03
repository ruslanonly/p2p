#!/bin/bash

# Запуск agent и pcap параллельно
/app/agent &
/app/pcap &

# Ожидание обоих процессов
wait
