#!/bin/bash

for version in 12c 19c 21c; do
  # 单点
  standalone_output_file="standalone_${version}.yaml"
  sed "s/{{VERSION}}/${version}/g;" standalone.tpl > ../standalone/${standalone_output_file}
done
