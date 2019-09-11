#! /bin/bash

for i in {1..1000}; do
    ./run_test.sh
    if [[ "$?" != "0" ]]; then
        exit 23
    fi
    echo "\n ${i} run done@@@@@"
done
