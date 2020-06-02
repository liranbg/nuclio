#!/usr/bin/env bash

set -ex

python -m pip install -r ./requirements/common.txt

PYTHON="$(python -V 2>&1)"

if [[ ${PYTHON} == "Python 2"* ]]; then
    python -m pip install futures
    python -m pip install -r ./requirements/python2.txt
else
    python -m pip install -r ./requirements/python3_6.txt
fi

python -m unittest -v test_wrapper
