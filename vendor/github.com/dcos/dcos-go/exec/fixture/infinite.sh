#!/bin/bash

cnt=0
while true; do
  echo "STDOUT $cnt"
  echo "STDERR $cnt" >&2
  sleep 1
  cnt=$((cnt+1))
done
