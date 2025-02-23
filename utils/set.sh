#!/bin/bash

# this script allows consult and set the breakers parameters

# flags -e is the environment, it can be local, ci, uat or production (default is local)

# The parameter are (all are optional). If the parameter is not provided the script will get every parameter value
# -m memory: the memory threshold in %
# -l latency: the latency threshold in ms
# -w window: the latency window size
# -p percentile: the latency percentile in %
# -w wait: the wait time in seconds
# -s set val: set the value of the parameter. In this case, only one parameter can be set at a time

usage() {
    echo "Usage: $0 [-e <env>] [-m <memory>] [-l <latency>] [-w <window>] [-p <percentile>] [-w <wait>] [-s <set> <val>] [-g <get>]" 1>&2
    exit 1
}

while getopts 'e:m:l:w:p:w:s:g:' flag; do
    case "${flag}" in
        e) env="${OPTARG}" ;;
        m) memory="${OPTARG}" ;;
        l) latency="${OPTARG}" ;;
        w) window="${OPTARG}" ;;
        p) percentile="${OPTARG}" ;;
        w) wait="${OPTARG}" ;;
        s) set="${OPTARG}" ;;
        *) usage ;;
    esac
done

# env han not been defined, then set it to local
if [ -z "$env" ]; then
    env="local"
    url="http://localhost:8080"
else
  # validate the environment (ci, uat, production)
  if [ "$env" != "local" ] && [ "$env" != "ci" ] && [ "$env" != "uat" ] && [ "$env" != "production" ]; then
      echo "Invalid environment"
      exit 1
  fi
  if [ "$env" == "local" ]; then
    url="http://localhost:8080"
  else
    # read the environment variable SPORT and LEAGUE
    sport=$SPORT
    league=$LEAGUE
    # build the URL
    url="http://${sport}-mc-${league}.sports-models-${env}.i.geniussports.com"
  fi
  fi

  # if -s flag is provided, set the value of the parameter
  if [ -n "$set" ]; then
      echo "setting $set"
      curl -X POST -H "Content-Type: application/json"  $url/$set/$val
      else
      if [ -n "$get" ]; then
          echo "getting $get"
          curl -X GET -H "Content-Type: application/json"  $url/$get
      else
          echo "getting all parameters"
          curl -X GET -H "Content-Type: application/json"  $url/breakers
      fi
  fi



