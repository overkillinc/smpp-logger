#!/bin/sh
# Install repository hooks by setting core.hooksPath for this repo
git config core.hooksPath .githooks && echo "git hooks installed (core.hooksPath=.githooks)"
