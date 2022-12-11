#!/usr/bin/env bash

go build
./einkaufsliste | tee -a log
