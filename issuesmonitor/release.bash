#!/bin/bash

./cross_compile.bash

for filename in issuesmonitor-*; do
    zip -r "$filename.zip" "$filename"
    rm "$filename"
done