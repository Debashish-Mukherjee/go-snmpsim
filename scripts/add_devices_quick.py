#!/usr/bin/env python3
"""
Quick script to add 100 devices to Zabbix
"""

import sys
sys.path.insert(0, '/home/debashish/trials/go-snmpsim/zabbix')

from zabbix_api_client import ZabbixAPIClient, ZabbixAPIError
import yaml

# Load config
with open('/home/debashish/trials/go-snmpsim/zabbix/zabbix_config.yaml', 'r') as f:
    config = yaml.safe_load(f)

print("🔗 Connecting to Zabbix...")
client = ZabbixAPIClient(
    config['zabbix_url'],
    config['zabbix_username'],
    config['zabbix_password']
)

print(f"📍 Zabbix Version: {client.get_version()}")

print("🔑 Authenticating...")
try:
    client.login()
    print("✅ Authentication successful!\n")
except ZabbixAPIError as e:
    print(f"❌ Authentication failed: {e}")
    sys.exit(1)

# Add 100 devices
print("📦 Adding 100 devices to Zabbix...\n")

snmp_port_start = config['snmp_port_start']
snmp_community = config['snmp_community']
snmp_version = config['snmp_version']

added = 0
skipped = 0
failed = 0

for device_num in range(1, 101):
    device_id = f"{device_num:03d}"
    hostname = f"cisco-iosxr-{device_id}"
    ip = "127.0.0.1"
    port = snmp_port_start + device_num - 1
    
    try:
        # Check if exists
        existing = client.get_host({"host": hostname})
        if existing:
            print(f"⊘ Device {device_num:3d}: {hostname:20s} (port {port}) - already exists")
            skipped += 1
            continue
        
        # Create host
        hostid = client.create_host(
            hostname=hostname,
            ip_address=ip,
            port=port,
            snmp_version=snmp_version,
            community=snmp_community
        )
        
        print(f"✅ Device {device_num:3d}: {hostname:20s} (port {port}) - added successfully")
        added += 1
        
    except Exception as e:
        print(f"❌ Device {device_num:3d}: {hostname:20s} (port {port}) - failed: {e}")
        failed += 1

print(f"\n" + "="*60)
print(f"📊 Summary:")
print(f"   Added:   {added}")
print(f"   Skipped: {skipped}")
print(f"   Failed:  {failed}")
print(f"   Total:   100")
print("="*60)

client.logout()
print("\n✅ Done!")
