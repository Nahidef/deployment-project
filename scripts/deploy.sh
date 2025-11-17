#!/bin/bash

echo "Deployment script"
case "$1" in
  canary)
    echo "Canary deployment..."
    ;;
  promote)
    echo "Promoting to production..."
    ;;
  rollback)
    echo "Rolling back..."
    ;;
  *)
    echo "Usage: $0 {canary|promote|rollback}"
    ;;
esac
