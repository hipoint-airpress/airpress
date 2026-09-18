#!/bin/sh

check_and_copy() {
    if [ ! -e /airpress/$1 ]; then
        mkdir -p /airpress/$1
        cp -Rf /app/$1/* /airpress/$1/
    fi
}

make_and_copy() {
    mkdir -p /airpress/$1
    cp -Rf /app/$1/* /airpress/$1/
}

make_and_copy 'resources/admin'
make_and_copy 'resources/template/common'
check_and_copy 'conf'
check_and_copy 'resources/template/theme'

