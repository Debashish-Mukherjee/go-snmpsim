#!/bin/bash
# Script to migrate files to new structure

# Copy store files
for file in oid_database.go oid_table.go oid_index_manager.go oid_template.go oid_device_mapping.go snmprec_loader.go snmpwalk_parser.go; do
    sed 's/^package main$/package store/' "$file" > "internal/store/${file#oid_}"
done

# Rename in store
cd internal/store
[ -f oid_database.go ] && mv oid_database.go database.go
[ -f oid_table.go ] && mv oid_table.go table.go
[ -f oid_index_manager.go ] && mv oid_index_manager.go index.go
[ -f oid_template.go ] && mv oid_template.go template.go
[ -f oid_device_mapping.go ] && mv oid_device_mapping.go mapping.go
[ -f snmprec_loader.go ] && mv snmprec_loader.go loader.go
[ -f snmpwalk_parser.go ] && mv snmpwalk_parser.go parser.go
cd ../..

echo "Store package files created"

# Copy agent files
for file in agent.go types.go; do
    sed 's/^package main$/package agent/' "$file" > "internal/agent/$file"
done
echo "Agent package files created"

# Copy engine files
for file in simulator.go dispatcher.go; do
    sed 's/^package main$/package engine/' "$file" > "internal/engine/$file"
done
echo "Engine package files created"

# Copy main
cp main.go cmd/snmpsim/main.go
echo "Main copied"

ls -la internal/store/
ls -la internal/agent/
ls -la internal/engine/
ls -la cmd/snmpsim/
